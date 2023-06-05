import { useState } from "react";
import Container from "@mui/system/Container";
import { useParams } from "react-router";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import Typography from "@mui/material/Typography";
import { useLoading } from "@/hooks/Loading";
import services from "@/injectServices";
import { useOrganization, useScenarios } from "@/services";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import AddUserDialog from "@/components/AddUserDialog";
import CreateButtonFab from "@/components/CreateButtonFab";
import { useUsers, useCreateUser } from "@/services";
import { type CreateUser, Role } from "@/models";
import ListOfUsers from "@/components/ListOfUsers";

function OrganizationDetailsPage() {
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

  const { scenarios } = useScenarios(
    services().organizationService,
    pageLoadingDispatcher,
    organizationId
  );

  const [createUserDialogOpen, setCreateUserDialogOpen] = useState(false);
  const { createUser } = useCreateUser(services().userService);

  const { users, refreshUsers } = useUsers(
    services().userService,
    pageLoadingDispatcher,
    organizationId
  );

  const handleCreateUserClick = () => {
    setCreateUserDialogOpen(true);
  };

  const handleValidateCreateUser = async (newUser: CreateUser) => {
    await createUser(newUser);
    await refreshUsers();
  };

  return (
    <>
      <DelayedLinearProgress loading={pageLoading} />
      <AddUserDialog
        open={createUserDialogOpen}
        setDialogOpen={setCreateUserDialogOpen}
        onValidate={handleValidateCreateUser}
        organizationId={organizationId}
        availableRoles={[Role.VIEWER, Role.BUILDER, Role.PUBLISHER, Role.ADMIN]}
        title="Add User"
      ></AddUserDialog>
      <Container
        sx={{
          maxWidth: "md",
          position: "relative",
        }}
      >
        <CreateButtonFab title="Add User" onClick={handleCreateUserClick} />

        {/* <div>organizationId: {organizationId}</div> */}
        {organization && (
          <>
            <Typography variant="h3">{organization.name}</Typography>
          </>
        )}
        {scenarios != null && (
          <>
            <Typography variant="h4">{scenarios.length} Scenarios</Typography>
            {scenarios.map((scenario) => (
              <Card key={scenario.scenariosId}>
                <CardContent>
                  <Typography
                    sx={{ fontSize: 14 }}
                    color="text.secondary"
                    gutterBottom
                  >
                    Scenario
                  </Typography>
                  <Typography variant="h5" component="div">
                    {scenario.name}
                  </Typography>
                  <Typography sx={{ mb: 1.5 }} color="text.secondary">
                    {scenario.createdAt.toDateString()}
                  </Typography>
                  <Typography variant="body2">
                    {scenario.description}
                  </Typography>
                </CardContent>
              </Card>
            ))}
          </>
        )}
        {users != null && (
          <ListOfUsers users={users} />
        )}
      </Container>
    </>
  );
}

export default OrganizationDetailsPage;
