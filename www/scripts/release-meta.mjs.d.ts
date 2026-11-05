export interface WoxRelease {
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
  body?: string;
  githubReleaseUrl?: string;
  changelogUrl?: string;
}

export function changelogPath(): string;
export function changelogZhPath(): string;
export function formatDateParts(isoDate: string): Pick<
  WoxRelease,
  | "year"
  | "month"
  | "day"
  | "monthNameEn"
  | "monthNameEnShort"
  | "dateLongEn"
  | "dateShortEn"
  | "dateLongZh"
  | "dateMonthZh"
>;
export function publicReleaseMeta(release: WoxRelease): WoxRelease;
export function parseChangelog(markdown?: string): WoxRelease[];
export function getStableReleases(): WoxRelease[];
export function getChineseReleases(): WoxRelease[];
export function getLatestRelease(): WoxRelease;
export function assertReadmeLatest(latest?: WoxRelease): void;
export function generateChangelogPages(): void;

export interface GitHubReleaseSnapshot {
  tag: string;
  publishedAt: string;
  body: string;
}

export interface ReleaseNavigationItem {
  tag: string;
  year: number;
  date: string;
  href: string;
}

export function readGitHubReleaseSnapshot(): GitHubReleaseSnapshot[];
export function refreshGitHubReleaseSnapshot(): Promise<GitHubReleaseSnapshot[]>;
export function buildReleaseNavigation(
  documentedReleases: WoxRelease[],
  githubReleases?: GitHubReleaseSnapshot[],
): ReleaseNavigationItem[];
