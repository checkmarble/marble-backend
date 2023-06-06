export enum PathFragment {
  Home = "",
  Login = "login",
  Organizations = "organizations",
  OrganizationDetails = "organizations",
  Users = "users",
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
  `/${PathFragment.OrganizationDetails}/${organizationId}`,
  Users: `/${PathFragment.Users}`,
};

export function isRouteRequireAuthenticatedUser(path: string) {
  // All pages but login
  return !path.startsWith(PageLink.Login);
}
