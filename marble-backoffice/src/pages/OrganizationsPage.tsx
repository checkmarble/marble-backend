import { useState } from "react";
import services from "@/injectServices";
import { Organization } from "@/models";
import { useAllOrganizations, useCreateOrganization } from "@/services";
import BusinessIcon from "@mui/icons-material/Business";
import AddIcon from "@mui/icons-material/Add";
import ListSubheader from "@mui/material/ListSubheader";
import Fab from "@mui/material/Fab";
import Avatar from "@mui/material/Avatar";
import Container from "@mui/material/Container";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemAvatar from "@mui/material/ListItemAvatar";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemText from "@mui/material/ListItemText";
import FormDialog from "@/components/FormDialog";
import { useLoading } from "@/hooks/Loading";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";

function OrganizationsPage() {
  const [pageLoading, pageLoadingDispatcher] = useLoading();

  const { allOrganizations, fetchAllOrganizations } = useAllOrganizations(
    services().organizationService,
    pageLoadingDispatcher
  );

  const [createOrgaDialogOpen, setCreateOrgaDialogOpen] = useState(false);

  const fakeOrganizations: Organization[] = [
    {
      organizationId: "someid",
      name: "Zorg",
      dateCreated: new Date(),
    },
  ];

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
    await fetchAllOrganizations();
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
        <Fab
          sx={{
            position: "absolute",
            top: "10px",
            right: "50px",
            paddingRight: "20px",
          }}
          color="primary"
          size="small"
          variant="extended"
          aria-label="add"
          onClick={handleCreateOrganizationClick}
        >
          <AddIcon sx={{ mr: 1 }} />
          New Organization
        </Fab>
        <List aria-label="organizations">
          <ListSubheader inset>
            {allOrganizations?.length} Organizations
          </ListSubheader>
          {(allOrganizations || fakeOrganizations).map((organization) => (
            <ListItem key={organization.organizationId}>
              <ListItemButton>
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
