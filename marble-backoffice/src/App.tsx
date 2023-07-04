//import "./App.css";
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
      <Box
        sx={{
          width: "100%",
          minHeight: "100vh",

          backgroundColor: "#fafafa",
          backgroundImage: "url(/background.svg)",
          backgroundRepeat: "no-repeat",
        }}
      >
        {authLoading ? (
          <LinearProgress color="secondary" />
        ) : displayPrivatePage ? (
          <>
            <BackOfficeAppBar />
            <Outlet />
          </>
        ) : (
          <Outlet />
        )}
      </Box>
    </AuthenticatedUserContext.Provider>
  );
}

export default App;
