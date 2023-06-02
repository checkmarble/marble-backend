import { useState } from "react";
import services from "@/injectServices";
import { PageLink } from "@/models";
import { useAllOrganizations, useCreateOrganization } from "@/services";
import BusinessIcon from "@mui/icons-material/Business";
import ListSubheader from "@mui/material/ListSubheader";
import Avatar from "@mui/material/Avatar";
import Container from "@mui/material/Container";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemAvatar from "@mui/material/ListItemAvatar";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemText from "@mui/material/ListItemText";
import FormDialog from "@/components/FormDialog";
import CreateButtonFab from "@/components/CreateButtonFab";
import { useLoading } from "@/hooks/Loading";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import { useNavigate } from "react-router";

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
        message="To create a new Organization, please enter it's name."
        inputLabel="Organization name"
        okTitle="Create"
        setDialogOpen={setCreateOrgaDialogOpen}
        onValidate={handleValidateCreateOrganization}
      ></FormDialog>
      <Container
        sx={{
          maxWidth: "md",
          position: "relative",
        }}
      >
        <CreateButtonFab title="New Organization" onClick={handleCreateOrganizationClick} />

        <List aria-label="organizations">
          <ListSubheader inset>
            {allOrganizations?.length} Organizations
          </ListSubheader>
          {(allOrganizations || []).map((organization) => (
            <ListItem key={organization.organizationId}>
              <ListItemButton
                onClick={() =>
                  handleOrganizationClick(organization.organizationId)
                }
              >
                <ListItemAvatar>
                  <Avatar>
                    <BusinessIcon />
                  </Avatar>
                </ListItemAvatar>
                <ListItemText
                  primary={organization.name}
                  secondary={organization.dateCreated.toDateString()}
                />
              </ListItemButton>
            </ListItem>
          ))}
        </List>
      </Container>
    </>
  );
}

export default OrganizationsPage;
