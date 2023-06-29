import { useState } from "react";
import services from "@/injectServices";
import { PageLink } from "@/models";
import { useAllOrganizations, useCreateOrganization } from "@/services";
import Container from "@mui/material/Container";
import FormDialog from "@/components/FormDialog";
import { useLoading } from "@/hooks/Loading";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import { useNavigate } from "react-router";
import Button from "@mui/material/Button";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import ListOfOrganizations from "@/components/ListOfOrganizations";
import AddIcon from "@mui/icons-material/Add";

function OrganizationsPage() {
  const [pageLoading, pageLoadingDispatcher] = useLoading();

  const { allOrganizations, refreshAllOrganizations } = useAllOrganizations(
    services().organizationService,
    pageLoadingDispatcher
  );

  const [createOrgaDialogOpen, setCreateOrgaDialogOpen] = useState(false);

  const { createOrganization } = useCreateOrganization(
    services().organizationService
  );

  const handleCreateOrganizationClick = () => {
    setCreateOrgaDialogOpen(true);
  };

  const handleValidateCreateOrganization = async (
    newOrganizationName: string
  ) => {
    await createOrganization(newOrganizationName);
    await refreshAllOrganizations();
  };

  const navigator = useNavigate();

  const handleOrganizationClick = (organizationId: string) => {
    navigator(PageLink.organizationDetails(organizationId));
  };

  return (
    <>
      <DelayedLinearProgress loading={pageLoading} />
      <FormDialog
        open={createOrgaDialogOpen}
        title="Create Organization"
        message="To create a new Organization, please enter it's name. ⚠ The Organization's name is used to create postgresql schema name."
        inputLabel="Organization name"
        okTitle="Create"
        setDialogOpen={setCreateOrgaDialogOpen}
        onValidate={handleValidateCreateOrganization}
      ></FormDialog>

      <Container sx={{ my: 1 }}>
        <Stack
          direction="column"
          justifyContent="flex-start"
          alignItems="center"
          spacing={2}
        >
          {/* Page content header */}
          <Stack
            direction="row"
            justifyContent="space-between"
            alignItems="center"
            spacing={2}
            sx={{
              minWidth: "100%",
            }}
          >
            {/* Title */}
            <Stack
              direction="row"
              justifyContent="flex-start"
              alignItems="center"
              spacing={2}
            >
              <Typography variant="h4" color={"secondary"}>
                {allOrganizations?.length}
              </Typography>
              <Typography variant="h4">Organizations</Typography>
            </Stack>

            {/* Organization Actions */}
            <Stack
              direction="row"
              justifyContent="flex-start"
              alignItems="center"
              spacing={2}
            >
              <Button
                variant="contained"
                startIcon={<AddIcon />}
                onClick={handleCreateOrganizationClick}
              >
                New Organisation
              </Button>
            </Stack>
          </Stack>

          {/* Page content details */}
          <Paper sx={{ minWidth: "100%" }}>
            <ListOfOrganizations
              organizations={allOrganizations}
              onOrganizationDetailClick={handleOrganizationClick}
            />
          </Paper>
        </Stack>
      </Container>
    </>
  );
}

export default OrganizationsPage;
