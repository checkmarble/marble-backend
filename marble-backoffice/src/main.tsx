import React from "react";
import { createBrowserRouter, RouterProvider } from "react-router-dom";
import ReactDOM from "react-dom/client";
import { backofficeRoutes } from "@/backofficeRoutes";
import { initializeFirebase } from "@/infra/firebase";
import {
  makeRepositories,
  type Repositories,
} from "./repositories/Repositories";
import { initializeServices } from "./injectServices";
import "./index.css";

const firebase = initializeFirebase();

const repositories: Repositories = makeRepositories(firebase);
initializeServices(repositories);

const router = createBrowserRouter(backofficeRoutes());

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <RouterProvider router={router} />
  </React.StrictMode>
);
