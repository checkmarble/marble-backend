import { useState, PropsWithChildren, useMemo } from "react";
import Button from "@mui/material/Button";
import TextField from "@mui/material/TextField";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";
import { CreateUser } from "@/models";
import { showLoader, useLoading } from "@/hooks/Loading";
import DelayedLinearProgress from "./DelayedLinearProgress";

interface AddUserFormDialogProps {
  open: boolean;
  setDialogOpen: (open: boolean) => void;
  onValidate: (user: CreateUser) => Promise<void>;
  organizationId: string;
}

export default function AddUserFormDialog(
  props: PropsWithChildren<AddUserFormDialogProps>
) {
  const initialUserViewModel = useMemo<CreateUser>(
    () => ({
      email: "",
      role: "MARBLE_ADMIN",
      organizationId: props.organizationId,
    }),
    [props.organizationId]
  );

  const [userViewModel, setUserViewModel] =
    useState<CreateUser>(initialUserViewModel);
  const [formLoading, formLoadingDispatcher] = useLoading();

  const handleClose = () => {
    props.setDialogOpen(false);
  };

  const handleValidate = async () => {
    await showLoader(formLoadingDispatcher, props.onValidate(userViewModel));
    setUserViewModel(initialUserViewModel);
    props.setDialogOpen(false);
  };

  const valid = userViewModel.email.trim() && userViewModel.role;

  const handleEmailChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setUserViewModel({
      ...userViewModel,
      email: event.target.value,
    });
  };

  const formDisable = formLoading;

  return (
    <div>
      <Dialog open={props.open} onClose={handleClose}>
        <DelayedLinearProgress loading={formLoading} />

        <DialogTitle>Create User</DialogTitle>
        <DialogContent>
          <DialogContentText>
            To create a new User, please enter it's email and role
          </DialogContentText>
          {props.children}
          <TextField
            autoFocus
            margin="dense"
            id="name"
            label="User's email"
            type="email"
            fullWidth
            variant="standard"
            value={userViewModel.email}
            onChange={handleEmailChange}
            disabled={formDisable}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={handleClose}>Cancel</Button>

          <Button onClick={handleValidate} disabled={!valid || formDisable}>
            Create
          </Button>
        </DialogActions>
      </Dialog>
    </div>
  );
}
