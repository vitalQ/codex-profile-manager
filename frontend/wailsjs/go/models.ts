export namespace config {
	
	export class Settings {
	    targetAuthPath: string;
	    activeProfileId?: string;
	    launchCodexAfterSwitch: boolean;
	    allowPlaintextExport: boolean;
	    enableSessionHistorySync: boolean;
	    theme: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.targetAuthPath = source["targetAuthPath"];
	        this.activeProfileId = source["activeProfileId"];
	        this.launchCodexAfterSwitch = source["launchCodexAfterSwitch"];
	        this.allowPlaintextExport = source["allowPlaintextExport"];
	        this.enableSessionHistorySync = source["enableSessionHistorySync"];
	        this.theme = source["theme"];
	    }
	}

}

export namespace detector {
	
	export class Diagnostics {
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
	
	    static createFrom(source: any = {}) {
	        return new Diagnostics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.targetAuthPath = source["targetAuthPath"];
	        this.targetDirExists = source["targetDirExists"];
	        this.targetDirWritable = source["targetDirWritable"];
	        this.authFileExists = source["authFileExists"];
	        this.authFileReadable = source["authFileReadable"];
	        this.managed = source["managed"];
	        this.activeProfileId = source["activeProfileId"];
	        this.activeProfileName = source["activeProfileName"];
	        this.activeFingerprint = source["activeFingerprint"];
	        this.warnings = source["warnings"];
	    }
	}

}

export namespace main {
	
	export class AuditEntryDTO {
	    id: string;
	    time: string;
	    action: string;
	    profileId?: string;
	    profileName?: string;
	    targetPath?: string;
	    result: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new AuditEntryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.time = source["time"];
	        this.action = source["action"];
	        this.profileId = source["profileId"];
	        this.profileName = source["profileName"];
	        this.targetPath = source["targetPath"];
	        this.result = source["result"];
	        this.message = source["message"];
	    }
	}
	export class CurrentStateDTO {
	    targetAuthPath: string;
	    exists: boolean;
	    managed: boolean;
	    profileId?: string;
	    profileName?: string;
	    fingerprint?: string;
	    size: number;
	    updatedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new CurrentStateDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.targetAuthPath = source["targetAuthPath"];
	        this.exists = source["exists"];
	        this.managed = source["managed"];
	        this.profileId = source["profileId"];
	        this.profileName = source["profileName"];
	        this.fingerprint = source["fingerprint"];
	        this.size = source["size"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class ProfileDTO {
	    id: string;
	    name: string;
	    mode: string;
	    homepage: string;
	    baseUrl?: string;
	    tags: string[];
	    note: string;
	    rawJson?: string;
	    fingerprint: string;
	    createdAt: string;
	    updatedAt: string;
	    lastUsedAt?: string;
	    storage?: profile.StorageInfo;
	
	    static createFrom(source: any = {}) {
	        return new ProfileDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.mode = source["mode"];
	        this.homepage = source["homepage"];
	        this.baseUrl = source["baseUrl"];
	        this.tags = source["tags"];
	        this.note = source["note"];
	        this.rawJson = source["rawJson"];
	        this.fingerprint = source["fingerprint"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.lastUsedAt = source["lastUsedAt"];
	        this.storage = this.convertValues(source["storage"], profile.StorageInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BootstrapData {
	    settings: config.Settings;
	    profiles: ProfileDTO[];
	    current: CurrentStateDTO;
	
	    static createFrom(source: any = {}) {
	        return new BootstrapData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.settings = this.convertValues(source["settings"], config.Settings);
	        this.profiles = this.convertValues(source["profiles"], ProfileDTO);
	        this.current = this.convertValues(source["current"], CurrentStateDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ImportProfileFromFileInput {
	    name: string;
	    mode: string;
	    homepage: string;
	    baseUrl: string;
	    tags: string[];
	    note: string;
	    filePath: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportProfileFromFileInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.mode = source["mode"];
	        this.homepage = source["homepage"];
	        this.baseUrl = source["baseUrl"];
	        this.tags = source["tags"];
	        this.note = source["note"];
	        this.filePath = source["filePath"];
	    }
	}
	export class ImportProfileFromRawInput {
	    name: string;
	    mode: string;
	    homepage: string;
	    baseUrl: string;
	    tags: string[];
	    note: string;
	    rawJson: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportProfileFromRawInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.mode = source["mode"];
	        this.homepage = source["homepage"];
	        this.baseUrl = source["baseUrl"];
	        this.tags = source["tags"];
	        this.note = source["note"];
	        this.rawJson = source["rawJson"];
	    }
	}
	export class ImportProfileInput {
	    name: string;
	    mode: string;
	    homepage: string;
	    baseUrl: string;
	    tags: string[];
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportProfileInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.mode = source["mode"];
	        this.homepage = source["homepage"];
	        this.baseUrl = source["baseUrl"];
	        this.tags = source["tags"];
	        this.note = source["note"];
	    }
	}
	
	export class SessionSyncDTO {
	    ran: boolean;
	    sourceProvider?: string;
	    targetProvider?: string;
	    scanned: number;
	    cloned: number;
	    skippedExists: number;
	    skippedTarget: number;
	    skippedInvalid: number;
	
	    static createFrom(source: any = {}) {
	        return new SessionSyncDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ran = source["ran"];
	        this.sourceProvider = source["sourceProvider"];
	        this.targetProvider = source["targetProvider"];
	        this.scanned = source["scanned"];
	        this.cloned = source["cloned"];
	        this.skippedExists = source["skippedExists"];
	        this.skippedTarget = source["skippedTarget"];
	        this.skippedInvalid = source["skippedInvalid"];
	    }
	}
	export class SwitchProfileResult {
	    profile: ProfileDTO;
	    current: CurrentStateDTO;
	    sessionSync: SessionSyncDTO;
	
	    static createFrom(source: any = {}) {
	        return new SwitchProfileResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = this.convertValues(source["profile"], ProfileDTO);
	        this.current = this.convertValues(source["current"], CurrentStateDTO);
	        this.sessionSync = this.convertValues(source["sessionSync"], SessionSyncDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UpdateProfileInput {
	    id: string;
	    name: string;
	    mode: string;
	    homepage: string;
	    baseUrl: string;
	    tags: string[];
	    note: string;
	    rawJson: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateProfileInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.mode = source["mode"];
	        this.homepage = source["homepage"];
	        this.baseUrl = source["baseUrl"];
	        this.tags = source["tags"];
	        this.note = source["note"];
	        this.rawJson = source["rawJson"];
	    }
	}

}

export namespace profile {
	
	export class StorageInfo {
	    type: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new StorageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.path = source["path"];
	    }
	}

}

