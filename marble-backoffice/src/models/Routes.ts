export enum PathFragment {
  Home = "home",
  Login = "login",
  Organizations = "organizations",
}

export const PageLink = {
  Home: `/${PathFragment.Home}`,
  Login: `/${PathFragment.Login}`,
  loginWithRedirect: (redirect?: string) => {
    const queryParams = redirect ? `?${new URLSearchParams({ redirect })}` : "";
    return `${PageLink.Login}${queryParams}`;
  },
  Organization: `/${PathFragment.Organizations}`,
};

export function isRouteRequireAuthenticatedUser(path: string) {
  // All pages but login
  return !path.startsWith(PageLink.Login);
}
