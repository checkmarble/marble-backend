export enum PathFragment {
  Home = "home",
  Organizations = "organizations",
}

export const PageLink = {
  Home: `/${PathFragment.Home}`,
  Organization: `/${PathFragment.Organizations}`,
} as const;
