import { Fragment, useState } from "react";
import services from "@/injectServices";
import { type CreateUser, Role, PageLink } from "@/models";
import { useUsers, useCreateUser } from "@/services";
import Container from "@mui/material/Container";
import { useLoading } from "@/hooks/Loading";
import AddUserDialog from "@/components/AddUserDialog";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import ListOfUsers from "@/components/ListOfUsers";
import { useNavigate } from "react-router-dom";
import Stack from "@mui/material/Stack";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";
import Button from "@mui/material/Button";
import AddIcon from "@mui/icons-material/Add";
import InlineOrganizationFromId from "@/components/InlineOrganizationFromId";
import { User } from "@/models";

function UsersPage() {
  const [pageLoading, pageLoadingDispatcher] = useLoading();
  const navigate = useNavigate();

  const { users, refreshUsers } = useUsers(
    services().userService,
    pageLoadingDispatcher
  );

  const marbleUsers =
    users?.filter((user) => user.organizationId === "") || null;
  const orgsWithUsers = new Set(
    users
      ?.filter((user) => user.organizationId !== "")
      .map((user) => user.organizationId)
  );

  type orgUsers = {
    orgId: string;
    users: User[] | null;
  };

  const orgUsersMapping: orgUsers[] = [];

  orgsWithUsers.forEach((orgId) => {
    orgUsersMapping.push({
      orgId: orgId,
      users:
        users !== null
          ? users.filter((user) => user.organizationId === orgId)
          : null,
    });
  });

  const [createUserDialogOpen, setCreateUserDialogOpen] = useState(false);

  const { createUser } = useCreateUser(services().userService);

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
        organizationId=""
        availableRoles={[Role.MARBLE_ADMIN]}
        title="Add Marble Admin"
      ></AddUserDialog>

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
                {marbleUsers?.length}
              </Typography>
              <Typography variant="h4">Marble users</Typography>
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
                onClick={handleCreateUserClick}
              >
                Add Marble Admin
              </Button>
            </Stack>
          </Stack>

          {/* Page content details */}
          <Paper sx={{ minWidth: "100%" }}>
            <ListOfUsers
              users={marbleUsers}
              onUserDetailClick={(userId) => {
                navigate(PageLink.userDetails(userId));
              }}
            />
          </Paper>

          {orgUsersMapping.map((oum) => {
            return (
              <Fragment key={oum.orgId}>
                <Stack
                  direction="row"
                  justifyContent="space-between"
                  alignItems="center"
                  spacing={2}
                  sx={{
                    minWidth: "100%",
                  }}
                >
                  <Stack
                    direction="row"
                    justifyContent="flex-start"
                    alignItems="center"
                    spacing={2}
                  >
                    <Typography variant="h4" color={"secondary"}>
                      {oum.users?.length}
                    </Typography>
                    <InlineOrganizationFromId
                      variant="h4"
                      organizationId={oum.orgId}
                    />
                    <Typography variant="h4">users</Typography>
                  </Stack>
                </Stack>
                <Paper sx={{ minWidth: "100%" }}>
                  <ListOfUsers
                    users={oum.users}
                    onUserDetailClick={(userId) => {
                      navigate(PageLink.userDetails(userId));
                    }}
                  />
                </Paper>
              </Fragment>
            );
          })}
        </Stack>
      </Container>
    </>
  );
}

export default UsersPage;
