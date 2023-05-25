import { createRoutesFromElements, Route } from "react-router-dom";
import { PathFragment } from "@/models";
import App from "@/App";
import ErrorPage from "@/pages/ErrorPage";
import OrganizationsPage from "@/pages/OrganizationsPage";
import OrganizationDetailsPage from "./pages/OrganizationDetailsPage";
import HomePage from "@/pages/HomePage";
import LoginPage from "@/pages/LoginPage";

export function backofficeRoutes() {
  return createRoutesFromElements(
    <Route path="/" element={<App />} errorElement={<ErrorPage />}>
      <Route path={PathFragment.Home} element={<HomePage />} />
      <Route path={PathFragment.Login} element={<LoginPage />} />
      <Route
        path={PathFragment.Organizations}
        element={<OrganizationsPage />}
      />
      <Route
        path={`/${PathFragment.OrganizationDetails}/:organizationId`}
        element={<OrganizationDetailsPage />}
      />
    </Route>
  );
}
