import { Fragment, useEffect, useMemo, useRef, useState, type DragEvent, type ReactNode, type UIEvent } from "react";
import { EventsOn } from "../wailsjs/runtime/runtime";
import "./App.css";
import codexBlossom from "./assets/images/codex-blossom.svg";
import { backend } from "./backend";
import type { AuditEntry, BootstrapData, Diagnostics, ProfileRecord, Settings } from "./types";

type PageKey = "providers" | "editor" | "diagnostics" | "settings";
type ImportMode = "raw" | "current" | "file";
type ProviderMode = "official" | "api_key";

type EditorState = {
  mode: "create" | "edit";
  providerMode: ProviderMode;
  importMode: ImportMode;
  id?: string;
  name: string;
  homepage: string;
  baseUrl: string;
  note: string;
  apiKey: string;
  rawJson: string;
  filePath: string;
};

const emptyEditor = (): EditorState => ({
  mode: "create",
  providerMode: "official",
  importMode: "raw",
  name: "",
  homepage: "",
  baseUrl: "",
  note: "",
  apiKey: "",
  rawJson: "{\n  \"auth_mode\": \"chatgpt\"\n}",
  filePath: "",
});

function App() {
  const [page, setPage] = useState<PageKey>("providers");
  const [data, setData] = useState<BootstrapData | null>(null);
  const [loading, setLoading] = useState(true);
  const [busyText, setBusyText] = useState("");
  const [errorText, setErrorText] = useState("");
  const [successText, setSuccessText] = useState("");
  const [settingsDraft, setSettingsDraft] = useState<Settings | null>(null);
  const [isSettingsDirty, setIsSettingsDirty] = useState(false);
  const [editor, setEditor] = useState<EditorState>(emptyEditor);
  const [diagnostics, setDiagnostics] = useState<Diagnostics | null>(null);
  const [auditLogs, setAuditLogs] = useState<AuditEntry[]>([]);
  const [draggingProfileId, setDraggingProfileId] = useState<string | null>(null);
  const [dragOverProfileId, setDragOverProfileId] = useState<string | null>(null);
  const [pendingDeleteProfile, setPendingDeleteProfile] = useState<ProfileRecord | null>(null);
  const jsonHighlightRef = useRef<HTMLPreElement | null>(null);

  useEffect(() => {
    void refresh();
  }, []);

  useEffect(() => {
    const unsubscribe = EventsOn("state:changed", () => {
      void refresh();
    });
    return () => {
      unsubscribe();
    };
  }, []);

  useEffect(() => {
    if (!data) {
      return;
    }
    if (page === "settings" && isSettingsDirty) {
      return;
    }
    setSettingsDraft(data.settings);
  }, [data, page, isSettingsDirty]);

  useEffect(() => {
    if (page !== "diagnostics") {
      return;
    }
    void refreshDiagnostics();
  }, [page, data?.current.profileId, data?.settings.targetAuthPath]);

  useEffect(() => {
    if (!successText) {
      return;
    }
    const timer = window.setTimeout(() => {
      setSuccessText("");
    }, 2600);
    return () => window.clearTimeout(timer);
  }, [successText]);

  useEffect(() => {
    if (!errorText || busyText) {
      return;
    }
    const timer = window.setTimeout(() => {
      setErrorText("");
    }, 4200);
    return () => window.clearTimeout(timer);
  }, [errorText, busyText]);

  const sortedProfiles = useMemo(() => data?.profiles ?? [], [data]);
  const currentProfile = useMemo(
    () => sortedProfiles.find((item) => item.id === data?.current.profileId) ?? null,
    [sortedProfiles, data?.current.profileId]
  );
  const editorPreviewJson = useMemo(
    () => (editor.providerMode === "api_key" ? buildAPIKeyAuthJSON(editor.apiKey) : editor.rawJson),
    [editor.apiKey, editor.providerMode, editor.rawJson]
  );
  const highlightedEditorJson = useMemo(() => syntaxHighlightJSON(editorPreviewJson), [editorPreviewJson]);
  const preferredTheme = useMemo(
    () => (page === "settings" && settingsDraft ? settingsDraft.theme : data?.settings.theme) ?? "system",
    [page, settingsDraft, data?.settings.theme]
  );

  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const applyTheme = () => {
      const resolvedTheme = resolveTheme(preferredTheme, media.matches);
      document.documentElement.dataset.theme = resolvedTheme;
      document.body.dataset.theme = resolvedTheme;
    };

    applyTheme();
    const handleChange = () => applyTheme();
    if (typeof media.addEventListener === "function") {
      media.addEventListener("change", handleChange);
      return () => media.removeEventListener("change", handleChange);
    }
    media.addListener(handleChange);
    return () => media.removeListener(handleChange);
  }, [preferredTheme]);

  useEffect(() => {
    if (page !== "editor") {
      return;
    }
    const handler = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setPage("providers");
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [page]);

  async function refresh() {
    setLoading(true);
    setErrorText("");
    try {
      const snapshot = await backend.bootstrap();
      setData(snapshot);
    } catch (error) {
      setErrorText(toMessage(error));
    } finally {
      setLoading(false);
    }
  }

  async function runAction(label: string, action: () => Promise<void>) {
    setBusyText(label);
    setErrorText("");
    setSuccessText("");
    try {
      await action();
    } catch (error) {
      setErrorText(toMessage(error));
    } finally {
      setBusyText("");
    }
  }

  function openCreateEditor() {
    setEditor(emptyEditor());
    setErrorText("");
    setSuccessText("");
    setPage("editor");
  }

  async function openEditEditor(profile: ProfileRecord) {
    setErrorText("");
    setSuccessText("");
    try {
      const detail = await backend.getProfile(profile.id);
      setEditor({
        mode: "edit",
        providerMode: normalizeProviderMode(detail.mode),
        importMode: "raw",
        id: detail.id,
        name: detail.name,
        homepage: detail.homepage,
        baseUrl: detail.baseUrl ?? "",
        note: detail.note,
        apiKey: extractAPIKey(detail.rawJson ?? ""),
        rawJson: detail.rawJson ?? "{\n  \"auth_mode\": \"chatgpt\"\n}",
        filePath: "",
      });
      setPage("editor");
    } catch (error) {
      setErrorText(toMessage(error));
    }
  }

  async function switchProfile(profile: ProfileRecord) {
    const currentMode = currentProfile ? normalizeProviderMode(currentProfile.mode) : null;
    const nextMode = normalizeProviderMode(profile.mode);
    const willSyncHistory =
      Boolean(data?.settings.enableSessionHistorySync) && currentMode !== null && currentMode !== nextMode;

    await runAction(willSyncHistory ? "正在切换供应商并同步历史会话..." : "正在切换供应商...", async () => {
      const result = await backend.switchProfile(profile.id);
      if (result.sessionSync.ran && result.sessionSync.cloned > 0) {
        setSuccessText(`已启用 ${profile.name}，并同步 ${result.sessionSync.cloned} 条历史会话`);
      } else {
        setSuccessText(`已启用 ${profile.name}`);
      }
      setPage("providers");
    });
  }

  async function duplicateProfile(profile: ProfileRecord) {
    await runAction("正在复制供应商...", async () => {
      const detail = await backend.getProfile(profile.id);
      await backend.importProfileFromRaw({
        name: `${profile.name} Copy`,
        mode: normalizeProviderMode(profile.mode),
        homepage: profile.homepage,
        baseUrl: profile.baseUrl ?? "",
        note: profile.note,
        tags: profile.tags,
        rawJson: detail.rawJson ?? "",
      });
      setSuccessText(`已复制 ${profile.name}`);
    });
  }

  async function refreshDiagnostics() {
    try {
      const [nextDiagnostics, nextLogs] = await Promise.all([backend.runDiagnostics(), backend.listAuditLogs()]);
      setDiagnostics(nextDiagnostics);
      setAuditLogs(nextLogs);
    } catch (error) {
      setErrorText(toMessage(error));
    }
  }

  async function moveProfile(sourceId: string, targetId: string) {
    if (!data || sourceId === targetId) {
      return;
    }

    const sourceIndex = data.profiles.findIndex((item) => item.id === sourceId);
    const targetIndex = data.profiles.findIndex((item) => item.id === targetId);
    if (sourceIndex === -1 || targetIndex === -1 || sourceIndex === targetIndex) {
      return;
    }

    const nextProfiles = [...data.profiles];
    const [moved] = nextProfiles.splice(sourceIndex, 1);
    const insertIndex = sourceIndex < targetIndex ? targetIndex - 1 : targetIndex;
    nextProfiles.splice(insertIndex, 0, moved);

    await runAction("正在更新供应商顺序...", async () => {
      await backend.reorderProfiles(nextProfiles.map((item) => item.id));
      setSuccessText("供应商顺序已更新");
    });
  }

  async function moveProfileToEnd(sourceId: string) {
    if (!data) {
      return;
    }

    const sourceIndex = data.profiles.findIndex((item) => item.id === sourceId);
    if (sourceIndex === -1 || sourceIndex === data.profiles.length - 1) {
      return;
    }

    const nextProfiles = [...data.profiles];
    const [moved] = nextProfiles.splice(sourceIndex, 1);
    nextProfiles.push(moved);

    await runAction("正在更新供应商顺序...", async () => {
      await backend.reorderProfiles(nextProfiles.map((item) => item.id));
      setSuccessText("供应商顺序已更新");
    });
  }

  function requestDeleteProfile(profile: ProfileRecord) {
    setPendingDeleteProfile(profile);
  }

  async function confirmDeleteProfile() {
    if (!pendingDeleteProfile) {
      return;
    }

    const profile = pendingDeleteProfile;
    setPendingDeleteProfile(null);
    await runAction("正在删除供应商...", async () => {
      await backend.deleteProfile(profile.id);
      setSuccessText("供应商已删除");
      if (editor.id === profile.id) {
        setEditor(emptyEditor());
        setPage("providers");
      }
    });
  }

  async function pickAuthPath() {
    const path = await backend.pickAuthPath();
    if (path && settingsDraft) {
      setIsSettingsDirty(true);
      setSettingsDraft({ ...settingsDraft, targetAuthPath: path });
    }
  }

  async function pickImportFile() {
    const path = await backend.pickImportFile();
    if (path) {
      setEditor((current) => ({ ...current, filePath: path, importMode: "file" }));
    }
  }

  function formatEditorJSON() {
    if (editor.providerMode === "api_key") {
      setSuccessText("API Key 模式下 auth.json 预览已自动格式化");
      setErrorText("");
      return;
    }
    try {
      const formatted = JSON.stringify(JSON.parse(editor.rawJson), null, 2);
      setEditor((current) => ({ ...current, rawJson: formatted }));
      setSuccessText("auth.json 已格式化");
      setErrorText("");
    } catch (error) {
      setErrorText(`JSON 格式化失败：${toMessage(error)}`);
    }
  }

  function syncJsonScroll(event: UIEvent<HTMLTextAreaElement>) {
    if (!jsonHighlightRef.current) {
      return;
    }
    jsonHighlightRef.current.scrollTop = event.currentTarget.scrollTop;
    jsonHighlightRef.current.scrollLeft = event.currentTarget.scrollLeft;
  }

  async function saveEditor() {
    const providerMode = normalizeProviderMode(editor.providerMode);
    const finalRawJson = providerMode === "api_key" ? buildAPIKeyAuthJSON(editor.apiKey) : editor.rawJson;
    const payload = {
      name: editor.name.trim(),
      mode: providerMode,
      homepage: editor.homepage.trim(),
      baseUrl: editor.baseUrl.trim(),
      note: editor.note.trim(),
      tags: [],
    };

    if (!payload.name) {
      setErrorText("供应商名称不能为空");
      return;
    }
    if (providerMode === "api_key") {
      if (!editor.apiKey.trim()) {
        setErrorText("API Key 不能为空");
        return;
      }
      if (!payload.baseUrl) {
        setErrorText("Base URL 不能为空");
        return;
      }
    }

    await runAction(editor.mode === "edit" ? "正在保存供应商..." : "正在创建供应商...", async () => {
      if (editor.mode === "edit" && editor.id) {
        await backend.updateProfile({
          id: editor.id,
          name: payload.name,
          mode: payload.mode,
          homepage: payload.homepage,
          baseUrl: payload.baseUrl,
          note: payload.note,
          tags: [],
          rawJson: finalRawJson,
        });
        setSuccessText("供应商已保存");
      } else if (editor.importMode === "current") {
        await backend.importProfileFromCurrent(payload);
        setSuccessText("已从当前 auth.json 导入供应商");
      } else if (editor.importMode === "file") {
        await backend.importProfileFromFile({
          ...payload,
          filePath: editor.filePath,
        });
        setSuccessText("已从本地文件导入供应商");
      } else {
        await backend.importProfileFromRaw({
          ...payload,
          rawJson: finalRawJson,
        });
        setSuccessText("供应商已创建");
      }

      setPage("providers");
      setEditor(emptyEditor());
    });
  }

  async function saveSettings() {
    if (!settingsDraft) {
      return;
    }

    await runAction("正在保存设置...", async () => {
      await backend.saveSettings(settingsDraft);
      setIsSettingsDirty(false);
      setSuccessText("设置已保存");
    });
  }

  if (loading && !data) {
    return <div className="app-shell loading-shell">正在加载 Codex Profile Manager…</div>;
  }

  return (
    <div className="app-shell">
      <div className="window-frame">
        <Header
          page={page}
          setPage={setPage}
          onCreate={openCreateEditor}
          currentName={currentProfile?.name ?? (data?.current.exists ? "未托管" : "未检测到 auth.json")}
        />

        {(successText || errorText) && (
          <div className="flash-stack">
            {successText && <Banner tone="success">{successText}</Banner>}
            {errorText && <Banner tone="danger">{errorText}</Banner>}
          </div>
        )}

        {busyText && <LoadingOverlay text={busyText} />}

        {pendingDeleteProfile && (
          <ConfirmDeleteModal
            profileName={pendingDeleteProfile.name}
            onCancel={() => setPendingDeleteProfile(null)}
            onConfirm={() => void confirmDeleteProfile()}
          />
        )}

        {page === "providers" && data && (
          <section className="providers-page">
            <div className="surface providers-surface">
              <div className="surface-header">
                <div>
                  <div className="surface-title">供应商列表</div>
                  <div className="surface-subtitle">当前启用：{currentProfile?.name ?? "未托管 auth.json"}</div>
                </div>
                <div className="surface-status">{sortedProfiles.length} 个供应商</div>
              </div>

              <div className="provider-list">
                {sortedProfiles.length === 0 ? (
                  <EmptyState onCreate={openCreateEditor} />
                ) : (
                  <>
                    {sortedProfiles.map((profile) => {
                      const showPlaceholder = draggingProfileId && draggingProfileId !== profile.id && dragOverProfileId === profile.id;
                      const active = data.current.profileId === profile.id;
                      return (
                        <Fragment key={profile.id}>
                          {showPlaceholder && (
                            <DropPlaceholder
                              onDragOver={(event) => {
                                event.preventDefault();
                                event.dataTransfer.dropEffect = "move";
                                if (dragOverProfileId !== profile.id) {
                                  setDragOverProfileId(profile.id);
                                }
                              }}
                              onDrop={(event) => {
                                event.preventDefault();
                                const sourceId = draggingProfileId;
                                setDraggingProfileId(null);
                                setDragOverProfileId(null);
                                if (sourceId) {
                                  void moveProfile(sourceId, profile.id);
                                }
                              }}
                            />
                          )}
                          <ProviderCard
                            profile={profile}
                            active={active}
                            dragging={draggingProfileId === profile.id}
                            dragOver={dragOverProfileId === profile.id}
                            onDragStart={() => setDraggingProfileId(profile.id)}
                            onDragEnd={() => {
                              setDraggingProfileId(null);
                              setDragOverProfileId(null);
                            }}
                            onDragEnter={() => {
                              if (draggingProfileId && draggingProfileId !== profile.id) {
                                setDragOverProfileId(profile.id);
                              }
                            }}
                            onDrop={() => {
                              const sourceId = draggingProfileId;
                              setDraggingProfileId(null);
                              setDragOverProfileId(null);
                              if (sourceId) {
                                void moveProfile(sourceId, profile.id);
                              }
                            }}
                            onSwitch={() => void switchProfile(profile)}
                            onEdit={() => void openEditEditor(profile)}
                            onDuplicate={() => void duplicateProfile(profile)}
                            onDelete={() => requestDeleteProfile(profile)}
                          />
                        </Fragment>
                      );
                    })}
                    {draggingProfileId && (
                      <div
                        className={`provider-drop-tail ${dragOverProfileId === "__tail__" ? "active" : ""}`}
                        onDragOver={(event) => {
                          event.preventDefault();
                          event.dataTransfer.dropEffect = "move";
                          if (dragOverProfileId !== "__tail__") {
                            setDragOverProfileId("__tail__");
                          }
                        }}
                        onDrop={(event) => {
                          event.preventDefault();
                          const sourceId = draggingProfileId;
                          setDraggingProfileId(null);
                          setDragOverProfileId(null);
                          if (sourceId) {
                            void moveProfileToEnd(sourceId);
                          }
                        }}
                      >
                        <DropPlaceholder end />
                      </div>
                    )}
                  </>
                )}
              </div>
            </div>
          </section>
        )}

        {page === "editor" && (
          <section className="editor-page">
            <button className="back-button" onClick={() => setPage("providers")}>←</button>
            <div className="editor-heading">{editor.mode === "edit" ? "编辑供应商" : "新增供应商"}</div>

            <div className="surface editor-surface">
              {editor.mode === "create" && (
                <div className="import-toggle-row mode-toggle-row">
                  <button
                    className={`import-pill ${editor.providerMode === "official" ? "active" : ""}`}
                    onClick={() =>
                      setEditor((current) => ({
                        ...current,
                        providerMode: "official",
                        importMode: current.importMode === "file" || current.importMode === "current" ? current.importMode : "raw",
                        apiKey: "",
                        baseUrl: "",
                      }))
                    }
                  >
                    官方账号
                  </button>
                  <button
                    className={`import-pill ${editor.providerMode === "api_key" ? "active" : ""}`}
                    onClick={() =>
                      setEditor((current) => ({
                        ...current,
                        providerMode: "api_key",
                        importMode: "raw",
                        rawJson: buildAPIKeyAuthJSON(current.apiKey),
                      }))
                    }
                  >
                    API Key
                  </button>
                </div>
              )}

              {editor.mode === "create" && editor.providerMode === "official" && (
                <div className="import-toggle-row">
                  <button
                    className={`import-pill ${editor.importMode === "raw" ? "active" : ""}`}
                    onClick={() => setEditor((current) => ({ ...current, importMode: "raw" }))}
                  >
                    粘贴 JSON
                  </button>
                  <button
                    className={`import-pill ${editor.importMode === "current" ? "active" : ""}`}
                    onClick={() => setEditor((current) => ({ ...current, importMode: "current" }))}
                  >
                    从当前 auth.json 导入
                  </button>
                  <button
                    className={`import-pill ${editor.importMode === "file" ? "active" : ""}`}
                    onClick={() => setEditor((current) => ({ ...current, importMode: "file" }))}
                  >
                    从本地文件导入
                  </button>
                </div>
              )}

              <div className="editor-grid two-columns">
                <Field label="供应商名称">
                  <input
                    className="input"
                    value={editor.name}
                    onChange={(event) => setEditor((current) => ({ ...current, name: event.target.value }))}
                  />
                </Field>
                <Field label="备注">
                  <input
                    className="input"
                    placeholder="例如：公司专用账号"
                    value={editor.note}
                    onChange={(event) => setEditor((current) => ({ ...current, note: event.target.value }))}
                  />
                </Field>
              </div>

              <Field label="官网链接">
                <input
                  className="input"
                  placeholder="https://chatgpt.com/codex"
                  value={editor.homepage}
                  onChange={(event) => setEditor((current) => ({ ...current, homepage: event.target.value }))}
                />
              </Field>

              {editor.providerMode === "api_key" && (
                <>
                  <Field label="API Key">
                    <input
                      className="input"
                      placeholder="请输入 API Key"
                      value={editor.apiKey}
                      onChange={(event) => setEditor((current) => ({ ...current, apiKey: event.target.value }))}
                    />
                  </Field>

                  <Field label="Base URL">
                    <input
                      className="input"
                      placeholder="https://example.com/v1"
                      value={editor.baseUrl}
                      onChange={(event) => setEditor((current) => ({ ...current, baseUrl: event.target.value }))}
                    />
                  </Field>
                </>
              )}

              {editor.providerMode === "api_key" && (
                <div className="info-banner">
                  切换该供应商时，会同时写入 <code>auth.json</code> 与同目录下的 <code>config.toml</code> 自定义 provider 配置。
                </div>
              )}

              {editor.mode === "create" && editor.providerMode === "official" && editor.importMode === "current" && (
                <div className="info-banner">保存时会从当前配置的目标 auth.json 直接导入内容。</div>
              )}

              {editor.mode === "create" && editor.providerMode === "official" && editor.importMode === "file" && (
                <div className="file-picker-row">
                  <input
                    className="input"
                    placeholder="请选择本地 auth.json 文件"
                    value={editor.filePath}
                    onChange={(event) => setEditor((current) => ({ ...current, filePath: event.target.value }))}
                  />
                  <button className="icon-button text-button" onClick={() => void pickImportFile()}>
                    选择文件
                  </button>
                </div>
              )}

              {(editor.providerMode === "official" && (editor.mode === "edit" || editor.importMode === "raw")) && (
                <Field label="auth.json (JSON) *">
                  <div className="json-editor-shell">
                    <pre
                      ref={jsonHighlightRef}
                      className="json-editor-highlight"
                      aria-hidden="true"
                      dangerouslySetInnerHTML={{ __html: highlightedEditorJson }}
                    />
                    <textarea
                      className="json-editor"
                      spellCheck={false}
                      value={editor.rawJson}
                      onScroll={syncJsonScroll}
                      onChange={(event) => setEditor((current) => ({ ...current, rawJson: event.target.value }))}
                    />
                  </div>
                </Field>
              )}

              {editor.providerMode === "api_key" && (
                <Field label="auth.json 预览">
                  <div className="json-editor-shell preview-only">
                    <pre
                      ref={jsonHighlightRef}
                      className="json-editor-highlight"
                      aria-hidden="true"
                      dangerouslySetInnerHTML={{ __html: highlightedEditorJson }}
                    />
                  </div>
                </Field>
              )}

              <div className="editor-footer">
                <div className="editor-footer-copy">
                  <div className="editor-footer-title">编辑完成后直接保存即可生效</div>
                  <div className="editor-footer-note">
                    {editor.providerMode === "api_key"
                      ? "API Key 模式会自动生成 auth.json，并同步维护 config.toml 中的 custom provider 配置。"
                      : "建议先格式化 JSON，方便检查结构和字段。"}
                  </div>
                </div>
                <div className="editor-footer-actions">
                  <button className="text-button" onClick={() => setPage("providers")}>
                    取消
                  </button>
                  <button className="text-button format-button" onClick={formatEditorJSON}>
                    格式化
                  </button>
                  <button className="save-button" onClick={() => void saveEditor()}>
                    保存
                  </button>
                </div>
              </div>
            </div>
          </section>
        )}

        {page === "diagnostics" && diagnostics && (
          <section className="secondary-page">
            <div className="surface secondary-surface">
              <div className="surface-title">运行诊断</div>
              <div className="stats-grid">
                <Stat label="目标目录存在" value={yesNo(diagnostics.targetDirExists)} />
                <Stat label="目标目录可写" value={yesNo(diagnostics.targetDirWritable)} />
                <Stat label="auth.json 存在" value={yesNo(diagnostics.authFileExists)} />
                <Stat label="当前已托管" value={yesNo(diagnostics.managed)} />
              </div>

              <div className="subsection-title">最近记录</div>
              <div className="audit-list">
                {auditLogs.slice(0, 8).map((entry) => (
                  <div key={entry.id} className="audit-row">
                    <div>
                      <div className="audit-action">{entry.action}</div>
                      <div className="audit-message">{entry.message}</div>
                    </div>
                    <div className="audit-time">{formatDate(entry.time)}</div>
                  </div>
                ))}
              </div>
            </div>
          </section>
        )}

        {page === "settings" && settingsDraft && (
          <section className="secondary-page">
            <div className="surface secondary-surface">
              <div className="surface-title">应用设置</div>
              <Field label="目标 auth.json 路径">
                <div className="file-picker-row">
                  <input
                    className="input"
                    value={settingsDraft.targetAuthPath}
                    onChange={(event) =>
                      {
                        setIsSettingsDirty(true);
                        setSettingsDraft({
                          ...settingsDraft,
                          targetAuthPath: event.target.value,
                        });
                      }
                    }
                  />
                  <button className="icon-button text-button" onClick={() => void pickAuthPath()}>
                    浏览
                  </button>
                </div>
              </Field>

              <ToggleRow
                label="切换时同步历史会话"
                checked={settingsDraft.enableSessionHistorySync}
                onChange={(checked) => {
                  setIsSettingsDirty(true);
                  setSettingsDraft({ ...settingsDraft, enableSessionHistorySync: checked });
                }}
              />

              <Field label="主题">
                <select
                  className="input"
                  value={settingsDraft.theme}
                  onChange={(event) => {
                    setIsSettingsDirty(true);
                    setSettingsDraft({ ...settingsDraft, theme: event.target.value });
                  }}
                >
                  <option value="system">跟随系统</option>
                  <option value="light">浅色</option>
                  <option value="dark">深色</option>
                </select>
              </Field>

              <div className="storage-note">
                当前版本会把 profile 中的 auth.json 原文保存在本地配置中；若使用 API Key 模式，会自动同步修改与 auth.json 同目录下的 config.toml。
              </div>

              <div className="editor-footer end-only">
                <button className="save-button" onClick={() => void saveSettings()}>
                  保存设置
                </button>
              </div>
            </div>
          </section>
        )}
      </div>
    </div>
  );
}

function Header(props: {
  page: PageKey;
  setPage: (page: PageKey) => void;
  onCreate: () => void;
  currentName: string;
}) {
  return (
    <header className="topbar">
      <div className="brand-row">
        <div className="brand-dot"><img className="brand-icon" src={codexBlossom} alt="" /></div>
        <div>
          <div className="brand-name">Codex Profile Manager</div>
          <div className="brand-subtitle">{props.currentName}</div>
        </div>
      </div>

      <div className="toolbar-row">
        <div className="segmented-toolbar">
          <ToolbarButton active={props.page === "providers"} onClick={() => props.setPage("providers")} title="供应商列表">
            <ListIcon />
          </ToolbarButton>
          <ToolbarButton active={props.page === "diagnostics"} onClick={() => props.setPage("diagnostics")} title="诊断">
            <PulseIcon />
          </ToolbarButton>
          <ToolbarButton active={props.page === "settings"} onClick={() => props.setPage("settings")} title="设置">
            <CogIcon />
          </ToolbarButton>
        </div>

        <button className="create-button" onClick={props.onCreate} title="新增供应商">
          +
        </button>
      </div>
    </header>
  );
}

function ProviderCard(props: {
  profile: ProfileRecord;
  active: boolean;
  dragging: boolean;
  dragOver: boolean;
  onDragStart: () => void;
  onDragEnd: () => void;
  onDragEnter: () => void;
  onDrop: () => void;
  onSwitch: () => void;
  onEdit: () => void;
  onDuplicate: () => void;
  onDelete: () => void;
}) {
  return (
    <div
      className={`provider-card ${props.active ? "active" : ""} ${props.dragging ? "dragging" : ""} ${props.dragOver ? "drag-over" : ""}`}
      draggable
      onDragStart={(event) => {
        event.dataTransfer.effectAllowed = "move";
        event.dataTransfer.setData("text/plain", props.profile.id);
        props.onDragStart();
      }}
      onDragEnd={props.onDragEnd}
      onDragOver={(event) => {
        event.preventDefault();
        event.dataTransfer.dropEffect = "move";
      }}
      onDragEnter={(event) => {
        event.preventDefault();
        props.onDragEnter();
      }}
      onDrop={(event) => {
        event.preventDefault();
        props.onDrop();
      }}
    >
      <div className="drag-handle" aria-hidden="true">
        <span />
        <span />
        <span />
        <span />
        <span />
        <span />
      </div>

      <div className="provider-body">
        <div className="provider-name-row">
          <div className="provider-name">{props.profile.name}</div>
          <div className={`provider-mode-pill ${normalizeProviderMode(props.profile.mode) === "api_key" ? "api-key" : ""}`}>
            {providerModeLabel(props.profile.mode)}
          </div>
        </div>
        <div className="provider-link">
          {normalizeProviderMode(props.profile.mode) === "api_key"
            ? props.profile.baseUrl || "未设置 Base URL"
            : props.profile.homepage || "未设置官网链接"}
        </div>
      </div>

      <div className="provider-actions">
        <button className={`enable-button ${props.active ? "active" : ""}`} onClick={props.onSwitch}>
          {props.active ? "已启用" : "启用"}
        </button>
        <IconActionButton title="编辑" onClick={props.onEdit}>
          <EditIcon />
        </IconActionButton>
        <IconActionButton title="复制" onClick={props.onDuplicate}>
          <CopyIcon />
        </IconActionButton>
        <IconActionButton title="删除" onClick={props.onDelete}>
          <TrashIcon />
        </IconActionButton>
      </div>
    </div>
  );
}

function DropPlaceholder(props: {
  end?: boolean;
  onDragOver?: (event: DragEvent<HTMLDivElement>) => void;
  onDrop?: (event: DragEvent<HTMLDivElement>) => void;
}) {
  return (
    <div
      className={`provider-placeholder ${props.end ? "end" : ""}`}
      onDragOver={props.onDragOver}
      onDrop={props.onDrop}
    >
      <div className="provider-placeholder-line" />
      <div className="provider-placeholder-chip">{props.end ? "释放到列表末尾" : "释放到这里"}</div>
    </div>
  );
}


function ConfirmDeleteModal(props: { profileName: string; onCancel: () => void; onConfirm: () => void }) {
  return (
    <div className="modal-overlay" onClick={props.onCancel}>
      <div
        className="confirm-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirm-delete-title"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="confirm-modal-copy">
          <div className="confirm-modal-title" id="confirm-delete-title">
            删除 {props.profileName}？
          </div>
          <div className="confirm-modal-text">
            将从列表中移除，但不会删除磁盘上已写入的 auth.json 文件。
          </div>
        </div>
        <div className="confirm-modal-actions">
          <button className="modal-button modal-button-secondary" onClick={props.onCancel}>
            取消
          </button>
          <button className="modal-button modal-button-danger" onClick={props.onConfirm}>
            删除
          </button>
        </div>
      </div>
    </div>
  );
}

function LoadingOverlay(props: { text: string }) {
  return (
    <div className="loading-overlay" role="status" aria-live="polite" aria-busy="true">
      <div className="loading-card">
        <div className="loading-spinner" aria-hidden="true" />
        <div className="loading-title">请稍候</div>
        <div className="loading-text">{props.text}</div>
      </div>
    </div>
  );
}

function EmptyState(props: { onCreate: () => void }) {
  return (
    <div className="empty-state">
      <div className="empty-title">还没有供应商</div>
      <div className="empty-copy">点击右上角 + ，新增一个 auth.json 供应商并开始快速切换。</div>
      <button className="save-button" onClick={props.onCreate}>
        新增供应商
      </button>
    </div>
  );
}

function Field(props: { label: string; children: ReactNode }) {
  return (
    <label className="field-block">
      <span className="field-label">{props.label}</span>
      {props.children}
    </label>
  );
}

function ToggleRow(props: { label: string; checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <label className="toggle-row">
      <span>{props.label}</span>
      <input type="checkbox" checked={props.checked} onChange={(event) => props.onChange(event.target.checked)} />
    </label>
  );
}

function Stat(props: { label: string; value: string }) {
  return (
    <div className="stat-card">
      <div className="stat-label">{props.label}</div>
      <div className="stat-value">{props.value}</div>
    </div>
  );
}

function Banner(props: { tone: "info" | "success" | "danger"; children: ReactNode }) {
  return <div className={`banner ${props.tone}`}>{props.children}</div>;
}

function ToolbarButton(props: { active: boolean; onClick: () => void; title: string; children: ReactNode }) {
  return (
    <button className={`toolbar-button ${props.active ? "active" : ""}`} onClick={props.onClick} title={props.title}>
      {props.children}
    </button>
  );
}

function IconActionButton(props: { title: string; onClick: () => void; children: ReactNode }) {
  return (
    <button className="icon-action" title={props.title} onClick={props.onClick}>
      {props.children}
    </button>
  );
}

function ListIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M8 6h12" />
      <path d="M8 12h12" />
      <path d="M8 18h12" />
      <path d="M3 6h.01" />
      <path d="M3 12h.01" />
      <path d="M3 18h.01" />
    </svg>
  );
}

function PulseIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 12h4l3-7 4 14 3-7h4" />
    </svg>
  );
}

function CogIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09a1.65 1.65 0 0 0-1-1.51 1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09a1.65 1.65 0 0 0 1.51-1 1.65 1.65 0 0 0-.33-1.82L4.21 7.2a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33h.01a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82v.01a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </svg>
  );
}

function EditIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 20h9" />
      <path d="M16.5 3.5a2.1 2.1 0 1 1 3 3L7 19l-4 1 1-4Z" />
    </svg>
  );
}

function CopyIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <rect x="9" y="9" width="11" height="11" rx="2" />
      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
    </svg>
  );
}

function TrashIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <path d="M3 6h18" />
      <path d="M8 6V4a1 1 0 0 1 1-1h6a1 1 0 0 1 1 1v2" />
      <path d="M19 6l-1 14a1 1 0 0 1-1 1H7a1 1 0 0 1-1-1L5 6" />
      <path d="M10 11v6" />
      <path d="M14 11v6" />
    </svg>
  );
}

function initials(value: string) {
  const cleaned = value.trim();
  if (!cleaned) {
    return "CC";
  }
  const parts = cleaned.split(/\s+/).filter(Boolean);
  if (parts.length === 1) {
    return cleaned.slice(0, 2).toUpperCase();
  }
  return `${parts[0][0] ?? ""}${parts[1][0] ?? ""}`.toUpperCase();
}

function yesNo(value: boolean) {
  return value ? "是" : "否";
}

function formatDate(value?: string) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString();
}

function toMessage(error: unknown) {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

function resolveTheme(theme: string, prefersDark: boolean) {
  if (theme === "dark") {
    return "dark";
  }
  if (theme === "light") {
    return "light";
  }
  return prefersDark ? "dark" : "light";
}

function normalizeProviderMode(mode: string): ProviderMode {
  return mode === "api_key" ? "api_key" : "official";
}

function providerModeLabel(mode: string) {
  return normalizeProviderMode(mode) === "api_key" ? "API Key" : "官方账号";
}

function buildAPIKeyAuthJSON(apiKey: string) {
  return JSON.stringify(
    {
      OPENAI_API_KEY: apiKey.trim(),
    },
    null,
    2
  );
}

function extractAPIKey(rawJson: string) {
  try {
    const parsed = JSON.parse(rawJson) as { OPENAI_API_KEY?: string };
    return typeof parsed.OPENAI_API_KEY === "string" ? parsed.OPENAI_API_KEY : "";
  } catch {
    return "";
  }
}

function syntaxHighlightJSON(raw: string) {
  const escaped = raw
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");

  return escaped.replace(
    /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g,
    (match) => {
      let className = "json-token-number";
      if (match.startsWith('"')) {
        className = match.endsWith(":") ? "json-token-key" : "json-token-string";
      } else if (match === "true" || match === "false") {
        className = "json-token-boolean";
      } else if (match === "null") {
        className = "json-token-null";
      }
      return `<span class="${className}">${match}</span>`;
    }
  );
}

export default App;
