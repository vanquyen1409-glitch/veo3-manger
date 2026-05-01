export namespace types {
	
	export class CDPStatus {
	    connected: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CDPStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.message = source["message"];
	    }
	}
	export class CreateVideoRequest {
	    prompt: string;
	    negativePrompt: string;
	    aspectRatio: string;
	    resolution: string;
	    duration: number;
	    tags: string[];
	
	    static createFrom(source: any = {}) {
	        return new CreateVideoRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prompt = source["prompt"];
	        this.negativePrompt = source["negativePrompt"];
	        this.aspectRatio = source["aspectRatio"];
	        this.resolution = source["resolution"];
	        this.duration = source["duration"];
	        this.tags = source["tags"];
	    }
	}
	export class Settings {
	    chromePath: string;
	    cdpPort: number;
	    selectorConfigPath: string;
	    outputDir: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chromePath = source["chromePath"];
	        this.cdpPort = source["cdpPort"];
	        this.selectorConfigPath = source["selectorConfigPath"];
	        this.outputDir = source["outputDir"];
	    }
	}
	export class Video {
	    id: string;
	    prompt: string;
	    negativePrompt: string;
	    aspectRatio: string;
	    resolution: string;
	    duration: number;
	    tags: string[];
	    status: string;
	    filePath: string;
	    errorMessage: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    completedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new Video(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.prompt = source["prompt"];
	        this.negativePrompt = source["negativePrompt"];
	        this.aspectRatio = source["aspectRatio"];
	        this.resolution = source["resolution"];
	        this.duration = source["duration"];
	        this.tags = source["tags"];
	        this.status = source["status"];
	        this.filePath = source["filePath"];
	        this.errorMessage = source["errorMessage"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.completedAt = this.convertValues(source["completedAt"], null);
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

