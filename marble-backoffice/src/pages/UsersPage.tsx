import { useState } from "react";
import services from "@/injectServices";
import { type CreateUser, Role } from "@/models";
import { useUsers, useCreateUser } from "@/services";
import Container from "@mui/material/Container";
import { useLoading } from "@/hooks/Loading";
import AddUserDialog from "@/components/AddUserDialog";
import CreateButtonFab from "@/components/CreateButtonFab";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import ListOfUsers from "@/components/ListOfUsers";

function UsersPage() {
  const [pageLoading, pageLoadingDispatcher] = useLoading();

  const { users, refreshUsers } = useUsers(
    services().userService,
    pageLoadingDispatcher
  );

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
      <Container
        sx={{
          maxWidth: "md",
          position: "relative",
        }}
      >
        <CreateButtonFab
          title="Add Marble Admin"
          onClick={handleCreateUserClick}
        />

        {users !== null && <ListOfUsers users={users} />}
      </Container>
    </>
  );
}

export default UsersPage;
