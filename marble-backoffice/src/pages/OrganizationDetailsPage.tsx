import { useState } from "react";
import { useParams } from "react-router";
import { useNavigate } from "react-router-dom";
import Card from "@mui/material/Card";
import Container from "@mui/system/Container";
import CardContent from "@mui/material/CardContent";
import Typography from "@mui/material/Typography";
import Button from "@mui/material/Button";
import Box from "@mui/material/Box";
import AddIcon from "@mui/icons-material/Add";
// import DeleteIcon from "@mui/icons-material/Delete";
import SendIcon from "@mui/icons-material/SendOutlined";

import { useLoading } from "@/hooks/Loading";
import services from "@/injectServices";
import {
  useOrganization,
  useScenarios,
  useUsers,
  useCreateUser,
  useDeleteOrganization,
} from "@/services";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import AddUserDialog from "@/components/AddUserDialog";
import { type CreateUser, Role, PageLink } from "@/models";
import ListOfUsers from "@/components/ListOfUsers";
import { DeleteForever } from "@mui/icons-material";
import AlertDialog from "@/components/AlertDialog";

function OrganizationDetailsPage() {
  const { organizationId } = useParams();
  const navigate = useNavigate();

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

  const [deleteOrgAlertDialogOpen, setDeleteOrgAlertDialogOpen] =
    useState(false);
  const { deleteOrganization } = useDeleteOrganization(
    services().organizationService
  );

  const handleDeleteOrgClick = () => {
    setDeleteOrgAlertDialogOpen(true);
  };
  const handleDeleteOrg = async () => {
    await deleteOrganization(organizationId);
    setDeleteOrgAlertDialogOpen(false);
    navigate(PageLink.Organizations);
  };

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

  const handleNavigateToIngestion = () => {
    navigate(PageLink.ingestion(organizationId));
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
      />
      <AlertDialog
        title="Confirm organization deletion"
        open={deleteOrgAlertDialogOpen}
        handleClose={() => {
          setDeleteOrgAlertDialogOpen(false);
        }}
        handleValidate={handleDeleteOrg}
      >
        <Typography variant="body1">
          Are you sure to delete {organization?.name} ? This action is
          destructive (no soft delete)
        </Typography>
      </AlertDialog>
      <Container
        sx={{
          maxWidth: "md",
          position: "relative",
        }}
      >
        <Typography variant="h3">{organization?.name}</Typography>
        <Box
          sx={{
            display: "flex",
            flexWrap: "wrap",
            justifyContent: "center",
            alignItems: "center",
            gap: 4,
          }}
        >
          <Button
            onClick={handleNavigateToIngestion}
            variant="text"
            startIcon={<SendIcon />}
          >
            Data Ingestion
          </Button>
          <Button
            onClick={handleCreateUserClick}
            variant="outlined"
            startIcon={<AddIcon />}
          >
            Add User
          </Button>
          <Button
            onClick={handleDeleteOrgClick}
            variant="contained"
            startIcon={<DeleteForever />}
            color="error"
          >
            Delete
          </Button>
        </Box>
        {scenarios != null && (
          <>
            <Typography variant="h4">{scenarios.length} Scenarios</Typography>
            {scenarios.map((scenario) => (
              <Card key={scenario.scenariosId} sx={{ mb: 2 }}>
                <CardContent>
                  <Typography
                    sx={{ fontSize: 14 }}
                    color="text.secondary"
                    gutterBottom
                  >
                    Scenario <code>{scenario.scenariosId}</code>
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
          <ListOfUsers
            users={users}
            onUserClick={(user) => {
              navigate(PageLink.userDetails(user.userId));
            }}
          />
        )}
      </Container>
    </>
  );
}

export default OrganizationDetailsPage;
