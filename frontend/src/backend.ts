import type {
  BootstrapData,
  Diagnostics,
  ImportProfileFromFileInput,
  ImportProfileFromRawInput,
  ImportProfileInput,
  ProfileRecord,
  Settings,
  SwitchProfileResult,
  UpdateProfileInput,
  AuditEntry,
} from "./types";

const app = () => window.go.main.App;

export const backend = {
  bootstrap(): Promise<BootstrapData> {
    return app().Bootstrap();
  },
  saveSettings(input: Settings): Promise<Settings> {
    return app().SaveSettings(input);
  },
  pickAuthPath(): Promise<string> {
    return app().PickAuthPath();
  },
  pickImportFile(): Promise<string> {
    return app().PickImportFile();
  },
  getProfile(id: string): Promise<ProfileRecord> {
    return app().GetProfile(id);
  },
  importProfileFromCurrent(input: ImportProfileInput): Promise<ProfileRecord> {
    return app().ImportProfileFromCurrentAuth(input);
  },
  importProfileFromFile(input: ImportProfileFromFileInput): Promise<ProfileRecord> {
    return app().ImportProfileFromFile(input);
  },
  importProfileFromRaw(input: ImportProfileFromRawInput): Promise<ProfileRecord> {
    return app().ImportProfileFromRaw(input);
  },
  updateProfile(input: UpdateProfileInput): Promise<ProfileRecord> {
    return app().UpdateProfile(input);
  },
  deleteProfile(id: string): Promise<void> {
    return app().DeleteProfile(id);
  },
  reorderProfiles(ids: string[]): Promise<ProfileRecord[]> {
    return app().ReorderProfiles(ids);
  },
  switchProfile(profileID: string): Promise<SwitchProfileResult> {
    return app().SwitchProfile(profileID);
  },
  listAuditLogs(): Promise<AuditEntry[]> {
    return app().ListAuditLogs();
  },
  runDiagnostics(): Promise<Diagnostics> {
    return app().RunDiagnostics();
  },
};
