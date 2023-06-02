import { useState } from "react";
import services from "@/injectServices";
import type { CreateUser } from "@/models";
import { useAllUsers, useCreateUser } from "@/services";
import BusinessIcon from "@mui/icons-material/Business";
import ListSubheader from "@mui/material/ListSubheader";
import Avatar from "@mui/material/Avatar";
import Container from "@mui/material/Container";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemAvatar from "@mui/material/ListItemAvatar";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemText from "@mui/material/ListItemText";
import { useLoading } from "@/hooks/Loading";
import AddUserDialog from "@/components/AddUserDialog";
import CreateButtonFab from "@/components/CreateButtonFab";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";

function UsersPage() {
  const [pageLoading, pageLoadingDispatcher] = useLoading();

  const { users, refreshUsers } = useAllUsers(
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
      ></AddUserDialog>
      <Container
        sx={{
          maxWidth: "md",
          position: "relative",
        }}
      >
        <CreateButtonFab
          title="New User"
          onClick={handleCreateUserClick}
        />

        <List aria-label="users">
          <ListSubheader inset>{users?.length} Users</ListSubheader>
          {(users || []).map((user) => (
            <ListItem key={user.userId}>
              <ListItemButton              >
                <ListItemAvatar>
                  <Avatar>
                    <BusinessIcon />
                  </Avatar>
                </ListItemAvatar>
                <ListItemText primary={user.email} secondary={user.role} />
              </ListItemButton>
            </ListItem>
          ))}
        </List>
      </Container>
    </>
  );
}

export default UsersPage;
