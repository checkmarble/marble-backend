import { Typography } from "@mui/material";
import { isRouteErrorResponse, useRouteError } from "react-router-dom";

function ErrorPage() {
  const error = useRouteError();
  console.error(error);

  const errorText = isRouteErrorResponse(error)
    ? `${error.status} : ${error.statusText}`
    : error instanceof Error
    ? error.message
    : `Unknown error: ${error}`;
  return (
    <>
      <Typography component="h1" variant="h4">
        Error
      </Typography>
      <p>{errorText}</p>
    </>
  );
}

export default ErrorPage;
