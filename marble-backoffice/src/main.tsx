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
import { buildEnvironment } from "./Environment";
import { ThemeProvider, createTheme } from "@mui/material";

const environment = buildEnvironment();
const firebase = initializeFirebase(
  environment.authEmulator,
  environment.firebaseOptions
);

const repositories: Repositories = makeRepositories(
  firebase,
  environment.marbleBackend
);
initializeServices(repositories);

const router = createBrowserRouter(backofficeRoutes());

/* Mui theming: add items here if custom properties need to be added to the theme
declare module '@mui/material/styles' {
  interface Theme {
  }
  // allow configuration using `createTheme`
  interface ThemeOptions {
  }
}
*/

const theme = createTheme({
  palette: {
    mode: "light",
    primary: {
      main: "#5a50fa",
    },
    secondary: {
      main: "#ff3d00",
    },
    background: {
      default: "#fafafa",
    },
  },
});

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <ThemeProvider theme={theme}>
      <RouterProvider router={router} />
    </ThemeProvider>
  </React.StrictMode>
);
