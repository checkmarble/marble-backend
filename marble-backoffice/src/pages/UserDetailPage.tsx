import services from "@/injectServices";
import { useNavigate, useParams } from "react-router-dom";
import Container from "@mui/material/Container";
import Button from "@mui/material/Button";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import DeleteForever from "@mui/icons-material/DeleteForever";
import AlertDialog from "@/components/AlertDialog";
import { useLoading } from "@/hooks/Loading";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import { useDeleteUser, useUser } from "@/services";
import { useState } from "react";
import Paper from "@mui/material/Paper";
import ReactJson from "react-json-view";

function UserDetailPage() {
  const { userId } = useParams();
  const navigate = useNavigate();

  const [pageLoading, pageLoadingDispatcher] = useLoading();

  const { user } = useUser(
    services().userService,
    pageLoadingDispatcher,
    userId
  );

  const [deleteUserAlertDialogOpen, setDeleteUserAlertDialogOpen] =
    useState(false);
  const { deleteUser } = useDeleteUser(services().userService);

  const handleDeleteUserClick = () => {
    setDeleteUserAlertDialogOpen(true);
  };
  const handleDeleteUser = async () => {
    await deleteUser(userId);
    setDeleteUserAlertDialogOpen(false);
    navigate(-1);
  };

  return (
    <>
      {/*
      <DelayedLinearProgress loading={pageLoading} />
      <Container
        sx={{
          maxWidth: "md",
        }}
      >
        <Stack direction="column" spacing={2}>
          <Typography variant="h3">User detail</Typography>
          {user && (
            <Card sx={{ padding: 2 }}>
              <CardContent>
                <Stack direction="column" spacing={2}>
                  <Typography variant="h5">Email: {user.email}</Typography>
                  <Typography variant="body1">Role: {user.role}</Typography>
                  <Typography color="text.secondary" gutterBottom>
                    UserId: <code>{user.userId}</code>
                  </Typography>
                  <Typography color="text.secondary">
                    OrgId: <code>{user.organizationId}</code>
                  </Typography>
                </Stack>
              </CardContent>
            </Card>
          )}
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
              onClick={handleDeleteUserClick}
              variant="contained"
              startIcon={<DeleteForever />}
              color="error"
            >
              Delete
            </Button>
          </Box>
        </Stack>

        <AlertDialog
          title="Confirm user deletion"
          open={deleteUserAlertDialogOpen}
          handleClose={() => {
            setDeleteUserAlertDialogOpen(false);
          }}
          handleValidate={handleDeleteUser}
        >
          <Typography variant="body1">
            Are you sure to delete this user ? This action is destructive (no
            soft delete)
          </Typography>
        </AlertDialog>
      </Container>

            */}

      <DelayedLinearProgress loading={pageLoading} />
      <AlertDialog
        title="Confirm user deletion"
        open={deleteUserAlertDialogOpen}
        handleClose={() => {
          setDeleteUserAlertDialogOpen(false);
        }}
        handleValidate={handleDeleteUser}
      >
        <Typography variant="body1">
          Are you sure to delete this user ? This action is destructive (no soft
          delete)
        </Typography>
      </AlertDialog>

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
              <Typography variant="h4">{user?.email}</Typography>
            </Stack>

            {/* User Actions */}
            <Stack
              direction="row"
              justifyContent="flex-start"
              alignItems="center"
              spacing={2}
            >
              <Button
                onClick={handleDeleteUserClick}
                variant="contained"
                startIcon={<DeleteForever />}
                color="error"
              >
                Delete
              </Button>
            </Stack>
          </Stack>

          {/* Page content details */}

          {user && (
            <Paper sx={{ minWidth: "100%", p: 2, fontSize: "0.8em" }}>
              <ReactJson
                src={user}
                name="user"
                collapsed={1}
                theme={"rjv-default"}
              />
            </Paper>
          )}
        </Stack>
      </Container>
    </>
  );
}

export default UserDetailPage;
