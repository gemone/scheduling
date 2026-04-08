export namespace main {
	
	export class ShiftEntry {
	    date: string;
	    person: string;
	    shift_type: string;
	
	    static createFrom(source: any = {}) {
	        return new ShiftEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.person = source["person"];
	        this.shift_type = source["shift_type"];
	    }
	}
	export class ScheduleRule {
	    day_shift_per_day: number;
	    night_shift_per_day: number;
	
	    static createFrom(source: any = {}) {
	        return new ScheduleRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.day_shift_per_day = source["day_shift_per_day"];
	        this.night_shift_per_day = source["night_shift_per_day"];
	    }
	}
	export class Vacation {
	    person_id: string;
	    date: string;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new Vacation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.person_id = source["person_id"];
	        this.date = source["date"];
	        this.type = source["type"];
	    }
	}
	export class Person {
	    id: string;
	    name: string;
	    min_total: number;
	    max_total: number;
	    max_day: number;
	    max_night: number;
	    day_shift_pos: number;
	    night_shift_pos: number;
	
	    static createFrom(source: any = {}) {
	        return new Person(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.min_total = source["min_total"];
	        this.max_total = source["max_total"];
	        this.max_day = source["max_day"];
	        this.max_night = source["max_night"];
	        this.day_shift_pos = source["day_shift_pos"];
	        this.night_shift_pos = source["night_shift_pos"];
	    }
	}
	export class MonthData {
	    people: Person[];
	    vacations: Vacation[];
	    rules: ScheduleRule;
	    schedule: ShiftEntry[];
	    pinned_days: string[];
	    year: number;
	    month: number;
	
	    static createFrom(source: any = {}) {
	        return new MonthData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.people = this.convertValues(source["people"], Person);
	        this.vacations = this.convertValues(source["vacations"], Vacation);
	        this.rules = this.convertValues(source["rules"], ScheduleRule);
	        this.schedule = this.convertValues(source["schedule"], ShiftEntry);
	        this.pinned_days = source["pinned_days"];
	        this.year = source["year"];
	        this.month = source["month"];
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

