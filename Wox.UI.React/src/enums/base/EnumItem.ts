export class EnumItem {
    public readonly code: string
    public readonly desc: string

    constructor(key: string, desc: string) {
        this.code = key
        this.desc = desc
    }
}
