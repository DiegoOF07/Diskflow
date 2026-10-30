export namespace model {
	
	export class Hit {
	    id: number;
	    name: string;
	    kind: string;
	    size: number;
	    diskSize: number;
	    flags: number;
	    files: number;
	    modified: number;
	    err?: string;
	    parentId: number;
	    dir: string;
	
	    static createFrom(source: any = {}) {
	        return new Hit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.size = source["size"];
	        this.diskSize = source["diskSize"];
	        this.flags = source["flags"];
	        this.files = source["files"];
	        this.modified = source["modified"];
	        this.err = source["err"];
	        this.parentId = source["parentId"];
	        this.dir = source["dir"];
	    }
	}
	export class ListQuery {
	    sort: string;
	    desc: boolean;
	    offset: number;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new ListQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sort = source["sort"];
	        this.desc = source["desc"];
	        this.offset = source["offset"];
	        this.limit = source["limit"];
	    }
	}
	export class Node {
	    id: number;
	    name: string;
	    kind: string;
	    size: number;
	    diskSize: number;
	    flags: number;
	    files: number;
	    modified: number;
	    err?: string;
	
	    static createFrom(source: any = {}) {
	        return new Node(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.size = source["size"];
	        this.diskSize = source["diskSize"];
	        this.flags = source["flags"];
	        this.files = source["files"];
	        this.modified = source["modified"];
	        this.err = source["err"];
	    }
	}
	export class Page {
	    items: Node[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new Page(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], Node);
	        this.total = source["total"];
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
	export class ScanInfo {
	    id: number;
	    status: string;
	    startedAt: number;
	    finishedAt: number;
	    files: number;
	    dirs: number;
	    bytes: number;
	    diskBytes: number;
	    errors: number;
	
	    static createFrom(source: any = {}) {
	        return new ScanInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.status = source["status"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.files = source["files"];
	        this.dirs = source["dirs"];
	        this.bytes = source["bytes"];
	        this.diskBytes = source["diskBytes"];
	        this.errors = source["errors"];
	    }
	}
	export class RootInfo {
	    id: number;
	    name: string;
	    path: string;
	    createdAt: number;
	    available: boolean;
	    index?: ScanInfo;
	    last?: ScanInfo;
	
	    static createFrom(source: any = {}) {
	        return new RootInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.createdAt = source["createdAt"];
	        this.available = source["available"];
	        this.index = this.convertValues(source["index"], ScanInfo);
	        this.last = this.convertValues(source["last"], ScanInfo);
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
	
	export class SearchResult {
	    hits: Hit[];
	    more: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hits = this.convertValues(source["hits"], Hit);
	        this.more = source["more"];
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

