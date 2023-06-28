import { useParams } from "react-router";
import Container from "@mui/system/Container";
import Typography from "@mui/material/Typography";
import Button from "@mui/material/Button";
import Box from "@mui/material/Box";
import BusinessIcon from "@mui/icons-material/Business";
import { useLoading } from "@/hooks/Loading";
import services from "@/injectServices";
import { useOrganization, useEditOrganization } from "@/services";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import TextField from "@mui/material/TextField";
import { Organization } from "@/models";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";

export default function OrganizationEditPage() {
  const { organizationId } = useParams();

  if (!organizationId) {
    throw Error("Organization Id is missing");
  }

  const [pageLoading, pageLoadingDispatcher] = useLoading();

  const { organization } = useOrganization(
    services().organizationService,
    pageLoadingDispatcher,
    organizationId
  );

  return (
    <>
      <DelayedLinearProgress loading={pageLoading} />
      <Container component="main" maxWidth="xs">
        <Paper sx={{ m: 2, p: 4 }}>
          <Stack alignItems="center" spacing={2}>
            {organization && (
              <>
                <BusinessIcon />
                <Typography component="h1" variant="h5">
                  Edit {organization.name}
                </Typography>
                <EditOrganizationForm organization={organization} />
              </>
            )}
          </Stack>
        </Paper>
      </Container>
    </>
  );
}

function EditOrganizationForm(props: { organization: Organization }) {
  const [saving, formSavingDispatcher] = useLoading();

  const { organizationViewModel, setOrganizationViewModel, saveOrganization } =
    useEditOrganization(
      services().organizationService,
      formSavingDispatcher,
      props.organization
    );

  const handleNameChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setOrganizationViewModel({
      ...organizationViewModel,
      name: event.target.value,
    });
  };

  const handleS3Change = (event: React.ChangeEvent<HTMLInputElement>) => {
    setOrganizationViewModel({
      ...organizationViewModel,
      exportScheduledExecutionS3: event.target.value,
    });
  };

  return (
    <Box>
      <DelayedLinearProgress loading={saving} />
      <Stack
        spacing={4}
        sx={{
          display: "block",
        }}
      >
        <TextField
          autoFocus
          fullWidth
          label="Display Name"
          variant="standard"
          value={organizationViewModel.name}
          onChange={handleNameChange}
          disabled={saving}
        />
        <TextField
          autoFocus
          fullWidth
          label="Export Scheduled Execution S3 Bucket"
          variant="standard"
          value={organizationViewModel.exportScheduledExecutionS3}
          onChange={handleS3Change}
          disabled={saving}
        />
        <TextField
          autoFocus
          fullWidth
          label="Database / Schema"
          variant="standard"
          value={props.organization.databaseName}
          disabled={true}
        />
        <Button
          onClick={saveOrganization}
          fullWidth
          variant="contained"
          sx={{ mt: 3 }}
          disabled={saving}
        >
          Save
        </Button>
      </Stack>
    </Box>
  );
}
