export interface Plugin {
  init: (context: PluginInitContext) => Promise<void>;
  query: (query: Query) => Promise<Result[]>;
}

export interface Query {
  /**
   * Raw query, this includes trigger keyword if it has
   * We didn't recommend use this property directly. You should always use Search property.
   */
  RawQuery: string;
  /**
   * Trigger keyword of a query. It can be empty if user is using global trigger keyword.
   */
  TriggerKeyword?: string;
  /**
   * Command part of a query.
   */
  Command?: string;
  /**
   * Search part of a query.
   */
  Search: string;
}

export interface Result {
  Id?: string;
  Title: string;
  SubTitle?: string;
  Icon: WoxImage;
  Score?: number;
  Action: () => Promise<boolean>;
}

export interface PluginInitContext {
  API: PublicAPI;
}

export interface PublicAPI {
  /**
   * Change Wox query
   */
  ChangeQuery: (query: string) => Promise<void>;

  /**
   * Hide Wox
   */
  HideApp: () => Promise<void>;

  /**
   * Show Wox
   */
  ShowApp: () => Promise<void>;

  /**
   * Show message box
   */
  ShowMsg: (title: string, description?: string, iconPath?: string) => Promise<void>;

  /**
   * Write log
   */
  Log: (msg: string) => Promise<void>;

  /**
   * Get translation of current language
   */
  GetTranslation: (key: string) => Promise<string>;
}

export type WoxImageType = "AbsolutePath" | "RelativeToPluginPath" | "Svg" | "Base64" | "Remote"

export interface WoxImage {
  ImageType: WoxImageType;
  ImageData: string;
}

export interface WoxImageCreator {
  FromAbsolutePath(path: string): WoxImage;

  FromRelativeToPluginPath(path: string): WoxImage;

  FromSvg(svg: string): WoxImage;

  FromBase64(base64: string): WoxImage;

  FromRemote(url: string): WoxImage;
}

export const WoxImageBuilder: WoxImageCreator = {
  FromAbsolutePath: (path: string) => {
    return {
      ImageType: "AbsolutePath",
      ImageData: path
    };
  },

  FromRelativeToPluginPath: (path: string) => {
    return {
      ImageType: "RelativeToPluginPath",
      ImageData: path
    };
  },

  FromSvg: (svg: string) => {
    return {
      ImageType: "Svg",
      ImageData: svg
    };
  },

  FromBase64: (base64: string) => {
    return {
      ImageType: "Base64",
      ImageData: base64
    };
  },

  FromRemote: (url: string) => {
    return {
      ImageType: "Remote",
      ImageData: url
    };
  }
};