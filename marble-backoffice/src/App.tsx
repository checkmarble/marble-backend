import "./App.css";
import { CssBaseline } from "@mui/material";
import { Outlet } from "react-router-dom";
import BackOfficeAppBar from "@/components/BackOfficeAppBar";

function App() {
  return (
    <>
      <CssBaseline />
      <BackOfficeAppBar />
      <Outlet />
    </>
  );
}

export default App;
