export enum PathFragment {
  Home = "",
  Login = "login",
  Organizations = "organizations",
  Users = "users",
  Ingestion = "ingestion",
  Decisions = "decisions",
  Scenarios = "scenarios",
}

export const PageLink = {
  Home: `/${PathFragment.Home}`,
  Login: `/${PathFragment.Login}`,
  loginWithRedirect: (redirect?: string) => {
    const queryParams = redirect ? `?${new URLSearchParams({ redirect })}` : "";
    return `${PageLink.Login}${queryParams}`;
  },
  Organizations: `/${PathFragment.Organizations}`,
  organizationDetails: (organizationId: string) =>
    `/${PathFragment.Organizations}/${organizationId}`,
  organizationEdit: (organizationId: string) =>
    `/${PathFragment.Organizations}/${organizationId}/edit`,
  Users: `/${PathFragment.Users}`,
  userDetails: (userId: string) => `/${PathFragment.Users}/${userId}`,
  ingestion: (organizationId: string) => `/${PathFragment.Ingestion}/${organizationId}`,
  decisions: (organizationId: string) => `/${PathFragment.Decisions}/${organizationId}`,
  scenarioDetailsPage: (scenarioId: string) => `/${PathFragment.Scenarios}/${scenarioId}`,
};

export function isRouteRequireAuthenticatedUser(path: string) {
  // All pages but login
  return !path.startsWith(PageLink.Login);
}
