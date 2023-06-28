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

          // Background pattern
          backgroundColor: "#fafafa",
          opacity: 0.8,
          backgroundImage:
            "radial-gradient(#5a50fa 0.5px, transparent 0.5px), radial-gradient(#5a50fa 0.5px, #fafafa 0.5px)",
          backgroundSize: "20px 20px",
          backgroundPosition: "0 0,10px 10px",
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
