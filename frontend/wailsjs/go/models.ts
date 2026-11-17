export namespace main {
	
	export class RecordingRequest {
	    mode: string;
	    window: number;
	    region?: model.CropRegion;
	    system_audio: boolean;
	    microphone: boolean;
	    mic_device: string;
	    fps: number;
	
	    static createFrom(source: any = {}) {
	        return new RecordingRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.window = source["window"];
	        this.region = this.convertValues(source["region"], model.CropRegion);
	        this.system_audio = source["system_audio"];
	        this.microphone = source["microphone"];
	        this.mic_device = source["mic_device"];
	        this.fps = source["fps"];
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

}

export namespace model {
	
	export class CaptureResult {
	    data: string;
	    width: number;
	    height: number;
	    // Go type: time
	    timestamp: any;
	
	    static createFrom(source: any = {}) {
	        return new CaptureResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.data = source["data"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
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
	export class CropRegion {
	    x: number;
	    y: number;
	    width: number;
	    height: number;
	
	    static createFrom(source: any = {}) {
	        return new CropRegion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}
	export class DisplayInfo {
	    id: number;
	    name: string;
	    width: number;
	    height: number;
	    x: number;
	    y: number;
	
	    static createFrom(source: any = {}) {
	        return new DisplayInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.x = source["x"];
	        this.y = source["y"];
	    }
	}
	export class SaveOptions {
	    path: string;
	    format: string;
	    quality: number;
	
	    static createFrom(source: any = {}) {
	        return new SaveOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.format = source["format"];
	        this.quality = source["quality"];
	    }
	}
	export class UploadProvider {
	    name: string;
	    enabled: boolean;
	    endpoint: string;
	    headers: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new UploadProvider(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.endpoint = source["endpoint"];
	        this.headers = source["headers"];
	    }
	}
	export class Settings {
	    default_save_path: string;
	    default_format: string;
	    default_quality: number;
	    hotkeys: Record<string, string>;
	    upload_providers: Record<string, UploadProvider>;
	    active_provider: string;
	    run_at_startup: boolean;
	    after_upload_action: string;
	    server_url: string;
	    api_key: string;
	    window_maximized: boolean;
	    window_width: number;
	    window_height: number;
	    record_system_audio: boolean;
	    record_microphone: boolean;
	    record_mic_device: string;
	    record_fps: number;
	    editor_prefs?: {[key: string]: any};

	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.default_save_path = source["default_save_path"];
	        this.default_format = source["default_format"];
	        this.default_quality = source["default_quality"];
	        this.hotkeys = source["hotkeys"];
	        this.upload_providers = this.convertValues(source["upload_providers"], UploadProvider, true);
	        this.active_provider = source["active_provider"];
	        this.run_at_startup = source["run_at_startup"];
	        this.after_upload_action = source["after_upload_action"];
	        this.server_url = source["server_url"];
	        this.api_key = source["api_key"];
	        this.window_maximized = source["window_maximized"];
	        this.window_width = source["window_width"];
	        this.window_height = source["window_height"];
	        this.record_system_audio = source["record_system_audio"];
	        this.record_microphone = source["record_microphone"];
	        this.record_mic_device = source["record_mic_device"];
	        this.record_fps = source["record_fps"];
	        this.editor_prefs = source["editor_prefs"];
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

}

export namespace recorder {
	
	export class AudioDevice {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new AudioDevice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class Window {
	    handle: number;
	    title: string;
	    app: string;
	    width: number;
	    height: number;
	
	    static createFrom(source: any = {}) {
	        return new Window(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.handle = source["handle"];
	        this.title = source["title"];
	        this.app = source["app"];
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}

}

export namespace upload {
	
	export class CompleteResponse {
	    public_url: string;
	    direct_url: string;
	
	    static createFrom(source: any = {}) {
	        return new CompleteResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.public_url = source["public_url"];
	        this.direct_url = source["direct_url"];
	    }
	}
	export class InitResponse {
	    file_id: string;
	    upload_url: string;
	
	    static createFrom(source: any = {}) {
	        return new InitResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file_id = source["file_id"];
	        this.upload_url = source["upload_url"];
	    }
	}

}

export namespace version {
	
	export class Info {
	    version: string;
	    build_time: string;
	    git_commit: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.build_time = source["build_time"];
	        this.git_commit = source["git_commit"];
	    }
	}

}

