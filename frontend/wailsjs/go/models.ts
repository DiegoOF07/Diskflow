export namespace model {
	
	export class Node {
	    id: number;
	    name: string;
	    kind: string;
	    size: number;
	    files: number;
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
	        this.files = source["files"];
	        this.err = source["err"];
	    }
	}

}

