import "./App.css";
import { CssBaseline } from "@mui/material";
import { Outlet } from "react-router-dom";
import AuthFence from "./components/AuthFence";
import BackOfficeAppBar from "@/components/BackOfficeAppBar";

function App() {
  return (
    <>
      <CssBaseline />
      <AuthFence>
        <BackOfficeAppBar />
        <Outlet />
      </AuthFence>
    </>
  );
}

export default App;
