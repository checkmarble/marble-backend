import { createRoutesFromElements, Route } from "react-router-dom";
import { PathFragment } from "@/models";
import App from "@/App";
import ErrorPage from "@/pages/ErrorPage";
import OrganizationsPage from "@/pages/OrganizationPage";
import HomePage from "@/pages/HomePage";

export function backofficeRoutes() {
  return createRoutesFromElements(
    <Route path="/" element={<App />} errorElement={<ErrorPage />}>
      <Route path={PathFragment.Home} element={<HomePage />} />
      <Route
        path={PathFragment.Organizations}
        element={<OrganizationsPage />}
      />
    </Route>
  );
}
