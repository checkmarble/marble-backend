import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import Container from "@mui/material/Container";
import Typography from "@mui/material/Typography";
import Paper from "@mui/material/Paper";
import { useCredentials } from "@/services";
import { useLoading } from "@/hooks/Loading";
import services from "@/injectServices";
import type { Credentials } from "@/models";

function HomePage() {
  const [pageLoading, pageLoadingDispatcher] = useLoading();

  const { credentials } = useCredentials(
    services().userService,
    pageLoadingDispatcher
  );

  return (
    <Container maxWidth="md">
      <DelayedLinearProgress loading={pageLoading} />
      {credentials && <CredentialsInfo credentials={credentials} />}
    </Container>
  );
}

const CredentialsInfo: React.FC<{ credentials: Credentials }> = ({
  credentials,
}) => {
  return (
    <Paper sx={{ p: 2 }}>
      <Typography variant="h6" gutterBottom>
        Your credentials
      </Typography>
      <Typography variant="subtitle1" gutterBottom>
        Role: {credentials.role}
      </Typography>

      <Typography variant="body1" gutterBottom>
        {credentials.actorIdentity.email && (
          <>Email: {credentials.actorIdentity.email}</>
        )}
        {credentials.actorIdentity.apiKeyName && (
          <>Api Key Name: {credentials.actorIdentity.email}</>
        )}
      </Typography>
    </Paper>
  );
};

export default HomePage;
