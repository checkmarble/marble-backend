import { useCallback, useState } from "react";
import services from "@/injectServices";
import Button from "@mui/material/Button";
import Container from "@mui/material/Container";
import Box from "@mui/material/Box";
import Avatar from "@mui/material/Avatar";
import LockOutlinedIcon from "@mui/icons-material/LockOutlined";
import Typography from "@mui/material/Typography";
import { SignInError } from "@/models";
import Alert from "@mui/material/Alert";

function Login() {
  const [errorMessage, setErrorMessage] = useState<string>("");
  const handleLogin = useCallback(() => {
    try {
      services().authenticationService.authenticationRepository.signIn();
    } catch (error) {
      if (error instanceof SignInError) {
        setErrorMessage(error.message);
      }
    }
  }, []);

  return (
    <Container maxWidth="xs">
      <Box
        sx={{
          marginTop: 8,
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
          sx={{ mt: 6, mb: 2 }}
        >
          Sign in using Google
        </Button>

        {errorMessage && <Alert severity="error">{errorMessage}</Alert>}
      </Box>
    </Container>
  );
}

export default Login;
