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

declare global {
  interface Window {
    go: {
      main: {
        App: {
          Bootstrap(): Promise<BootstrapData>;
          GetSettings(): Promise<Settings>;
          SaveSettings(input: Settings): Promise<Settings>;
          PickAuthPath(): Promise<string>;
          PickImportFile(): Promise<string>;
          GetProfile(id: string): Promise<ProfileRecord>;
          ListProfiles(): Promise<ProfileRecord[]>;
          ImportProfileFromCurrentAuth(input: ImportProfileInput): Promise<ProfileRecord>;
          ImportProfileFromFile(input: ImportProfileFromFileInput): Promise<ProfileRecord>;
          ImportProfileFromRaw(input: ImportProfileFromRawInput): Promise<ProfileRecord>;
          UpdateProfile(input: UpdateProfileInput): Promise<ProfileRecord>;
          DeleteProfile(id: string): Promise<void>;
          ReorderProfiles(ids: string[]): Promise<ProfileRecord[]>;
          SwitchProfile(profileID: string): Promise<SwitchProfileResult>;
          ListAuditLogs(): Promise<AuditEntry[]>;
          RunDiagnostics(): Promise<Diagnostics>;
        };
      };
    };
  }
}

export {};
