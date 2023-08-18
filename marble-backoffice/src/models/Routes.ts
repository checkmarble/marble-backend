export enum PathFragment {
  Home = "",
  Login = "login",
  Organizations = "organizations",
  Users = "users",
  Ingestion = "ingestion",
  Decisions = "decisions",
  Scenarios = "scenarios",
  AstEditor = "ast-editor",
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
  ingestion: (organizationId: string) =>
    `/${PathFragment.Ingestion}/${organizationId}`,
  decisions: (organizationId: string) =>
    `/${PathFragment.Decisions}/${organizationId}`,
  scenarioDetailsPage: (scenarioId: string, iterationId: string | null) => {
    const queryParams = iterationId
      ? `?${new URLSearchParams({ "iteration-id": iterationId })}`
      : "";
    return `/${PathFragment.Scenarios}/${scenarioId}${queryParams}`;
  },
  editTrigger: (scenarioId: string, iterationId: string) =>
    `/${PathFragment.AstEditor}/${scenarioId}/${iterationId}/trigger`,
  editRule: (scenarioId: string, iterationId: string, ruleId: string) =>
    `/${PathFragment.AstEditor}/${scenarioId}/${iterationId}/rule/${ruleId}`,
};

export function isRouteRequireAuthenticatedUser(path: string) {
  // All pages but login
  return !path.startsWith(PageLink.Login);
}
