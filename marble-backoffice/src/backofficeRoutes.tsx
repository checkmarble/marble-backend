import { createRoutesFromElements, Route } from "react-router-dom";
import { PathFragment } from "@/models";
import App from "@/App";
import ErrorPage from "@/pages/ErrorPage";
import OrganizationsPage from "@/pages/OrganizationsPage";
import UsersPage from "@/pages/UsersPage";
import OrganizationDetailsPage from "./pages/OrganizationDetailsPage";
import OrganizationEditPage from "./pages/OrganizationEditPage";
import HomePage from "@/pages/HomePage";
import LoginPage from "@/pages/LoginPage";
import IngestionPage from "./pages/IngestionPage";
import UserDetailPage from "./pages/UserDetailPage";
import DecisionsPage from "./pages/DecisionsPage";
import ScenarioDetailsPage from "./pages/ScenarioDetailsPage";
import AstEditorPage from "./pages/AstEditorPage";

export function backofficeRoutes() {
  return createRoutesFromElements(
    <Route path="/" element={<App />} errorElement={<ErrorPage />}>
      <Route path={PathFragment.Home} element={<HomePage />} />
      <Route path={PathFragment.Login} element={<LoginPage />} />
      <Route
        path={PathFragment.Organizations}
        element={<OrganizationsPage />}
      />
      <Route path={PathFragment.Users} element={<UsersPage />} />
      <Route
        path={`/${PathFragment.Users}/:userId`}
        element={<UserDetailPage />}
      />
      <Route
        path={`/${PathFragment.Organizations}/:organizationId`}
        element={<OrganizationDetailsPage />}
      />
      <Route
        path={`/${PathFragment.Organizations}/:organizationId/edit`}
        element={<OrganizationEditPage />}
      />
      <Route
        path={`/${PathFragment.Ingestion}/:organizationId`}
        element={<IngestionPage />}
      />
      <Route
        path={`/${PathFragment.Decisions}/:organizationId`}
        element={<DecisionsPage />}
      />
      <Route
        path={`/${PathFragment.Scenarios}/:scenarioId`}
        element={<ScenarioDetailsPage />}
      />
      <Route
        path={`/${PathFragment.AstEditor}/:scenarioId/:iterationId/trigger`}
        element={<AstEditorPage />}
      />
      <Route
        path={`/${PathFragment.AstEditor}/:scenarioId/:iterationId/rule/:ruleId`}
        element={<AstEditorPage />}
      />
    </Route>
  );
}
