import "./App.css";
import CssBaseline from "@mui/material/CssBaseline";
import Box from "@mui/material/Box";
import LinearProgress from "@mui/material/LinearProgress";
import { Outlet } from "react-router-dom";
import { AuthenticatedUserContext, useAuthentication } from "./services";
import services from "@/injectServices";
import BackOfficeAppBar from "@/components/BackOfficeAppBar";

function App() {
  const { user, authLoading, displayPrivatePage } = useAuthentication(
    services().authenticationService
  );

  return (
    <AuthenticatedUserContext.Provider value={user}>
      <CssBaseline />
      {authLoading ? (
        <Box sx={{ width: "100%" }}>
          <LinearProgress />
        </Box>
      ) : displayPrivatePage ? (
        <>
          <BackOfficeAppBar />
          <Outlet />
        </>
      ) : (
        <Outlet />
      )}
    </AuthenticatedUserContext.Provider>
  );
}

export default App;
