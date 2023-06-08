import services from "@/injectServices";
import { useDeleteUser, useUser } from "@/services";
import Container from "@mui/material/Container";
import { useLoading } from "@/hooks/Loading";
import DelayedLinearProgress from "@/components/DelayedLinearProgress";
import { useNavigate, useParams } from "react-router-dom";
import {
  Box,
  Button,
  Card,
  CardContent,
  Stack,
  Typography,
} from "@mui/material";
import { DeleteForever } from "@mui/icons-material";
import AlertDialog from "@/components/AlertDialog";
import { useState } from "react";

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
    </>
  );
}

export default UserDetailPage;
