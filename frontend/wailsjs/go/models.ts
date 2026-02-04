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
	
	export class User {
	    id: string;
	    email: string;
	    name: string;
	    picture: string;
	    username: string;
	
	    static createFrom(source: any = {}) {
	        return new User(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.email = source["email"];
	        this.name = source["name"];
	        this.picture = source["picture"];
	        this.username = source["username"];
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

