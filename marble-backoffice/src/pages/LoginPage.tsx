import { useCallback, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { SignInError } from "@/models";
import { useSignIn } from "@/services";
import services from "@/injectServices";
import Button from "@mui/material/Button";
import Box from "@mui/material/Box";
import Avatar from "@mui/material/Avatar";
import LockOutlinedIcon from "@mui/icons-material/LockOutlined";
import Typography from "@mui/material/Typography";
import Alert from "@mui/material/Alert";
import Grid from "@mui/material/Grid";
import Paper from "@mui/material/Paper";

function LoginPage() {
  const [searchParams] = useSearchParams();

  const [errorMessage, setErrorMessage] = useState<string>("");
  const { signIn } = useSignIn(
    services().authenticationService,
    searchParams.get("redirect")
  );

  const handleLogin = useCallback(async () => {
    try {
      await signIn();
    } catch (error) {
      if (error instanceof SignInError) {
        setErrorMessage(`${error.message}: ${error.from.message}`);
        throw error.from;
      }
    }
  }, [signIn]);

  return (
    <Grid
      container
      direction="row"
      justifyContent="center"
      alignItems="center"
      sx={{
        minHeight: "100vh",
      }}
    >
      <Grid item maxWidth="xs">
        <Paper>
          <Box
            py={2}
            px={8}
            sx={{
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
            }}
          >
            <Avatar sx={{ m: 1, bgcolor: "secondary.main" }}>
              <LockOutlinedIcon />
            </Avatar>

            <Typography component="h1" variant="h5">
              Marble BackOffice
            </Typography>

            <Button
              onClick={handleLogin}
              fullWidth
              variant="contained"
              sx={{ m: 2 }}
              data-testid="login-button"
            >
              Sign in using Google
            </Button>

            {errorMessage && <Alert severity="error">{errorMessage}</Alert>}
          </Box>
        </Paper>
      </Grid>
    </Grid>
  );
}

export default LoginPage;
