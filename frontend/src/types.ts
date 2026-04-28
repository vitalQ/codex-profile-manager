export type Settings = {
  targetAuthPath: string;
  activeProfileId?: string;
  launchCodexAfterSwitch: boolean;
  allowPlaintextExport: boolean;
  enableSessionHistorySync: boolean;
  theme: "system" | "dark" | "light" | string;
};

export type ProfileRecord = {
  id: string;
  name: string;
  mode: "official" | "api_key" | string;
  homepage: string;
  baseUrl?: string;
  tags: string[];
  note: string;
  rawJson?: string;
  fingerprint: string;
  createdAt: string;
  updatedAt: string;
  lastUsedAt?: string;
};

export type CurrentState = {
  targetAuthPath: string;
  exists: boolean;
  managed: boolean;
  profileId?: string;
  profileName?: string;
  fingerprint?: string;
  size: number;
  updatedAt?: string;
};

export type AuditEntry = {
  id: string;
  time: string;
  action: string;
  profileId?: string;
  profileName?: string;
  targetPath?: string;
  result: string;
  message: string;
};

export type Diagnostics = {
  targetAuthPath: string;
  targetDirExists: boolean;
  targetDirWritable: boolean;
  authFileExists: boolean;
  authFileReadable: boolean;
  managed: boolean;
  activeProfileId?: string;
  activeProfileName?: string;
  activeFingerprint?: string;
  warnings: string[];
};

export type BootstrapData = {
  settings: Settings;
  profiles: ProfileRecord[];
  current: CurrentState;
};

export type ImportProfileInput = {
  name: string;
  mode: "official" | "api_key" | string;
  homepage: string;
  baseUrl: string;
  tags: string[];
  note: string;
};

export type ImportProfileFromFileInput = ImportProfileInput & {
  filePath: string;
};

export type ImportProfileFromRawInput = ImportProfileInput & {
  rawJson: string;
};

export type UpdateProfileInput = {
  id: string;
  name: string;
  mode: "official" | "api_key" | string;
  homepage: string;
  baseUrl: string;
  tags: string[];
  note: string;
  rawJson: string;
};

export type SwitchProfileResult = {
  profile: ProfileRecord;
  current: CurrentState;
  sessionSync: {
    ran: boolean;
    sourceProvider?: string;
    targetProvider?: string;
    scanned: number;
    cloned: number;
    skippedExists: number;
    skippedTarget: number;
    skippedInvalid: number;
  };
};
