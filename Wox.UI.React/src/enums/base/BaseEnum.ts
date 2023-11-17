import {EnumItem} from "./EnumItem.ts";

export abstract class BaseEnum {
    // Use a separate variable to store enum values
    private static values: { [key: string]: EnumItem } = {};

    public static getDesc(key: string): string {
        const item = this.values[key];
        return item ? item.desc : key;
    }

    public static getAll(): EnumItem[] {
        return Object.values(this.values);
    }

    // Helper method to define enum values
    protected static define(key: string, desc: string): EnumItem {
        const item = new EnumItem(key, desc);
        this.values[key] = item;
        return item;
    }

}
