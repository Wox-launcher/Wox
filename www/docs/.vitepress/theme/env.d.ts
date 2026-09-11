interface WoxLatestRelease {
  version: string;
  tag: string;
  date: string;
  year: number;
  month: number;
  day: number;
  monthNameEn: string;
  monthNameEnShort: string;
  dateLongEn: string;
  dateShortEn: string;
  dateLongZh: string;
  dateMonthZh: string;
  highlight: string;
  githubReleaseUrl: string;
  changelogUrl: string;
}

declare const __WOX_LATEST_RELEASE__: WoxLatestRelease;
