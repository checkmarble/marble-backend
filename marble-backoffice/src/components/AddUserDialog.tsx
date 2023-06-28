import { useState, PropsWithChildren, useMemo } from "react";
import Button from "@mui/material/Button";
import TextField from "@mui/material/TextField";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import InputLabel from "@mui/material/InputLabel";
import Select, { type SelectChangeEvent } from "@mui/material/Select";
import MenuItem from "@mui/material/MenuItem";
import { type CreateUser, Role, adaptRole } from "@/models";
import DelayedLinearProgress from "./DelayedLinearProgress";
import { showLoader, useLoading } from "@/hooks/Loading";

interface AddUserFormDialogProps {
  open: boolean;
  availableRoles: Role[];
  setDialogOpen: (open: boolean) => void;
  onValidate: (user: CreateUser) => Promise<void>;
  organizationId: string;
  title: string;
}

export default function AddUserFormDialog(
  props: PropsWithChildren<AddUserFormDialogProps>
) {
  const initialUserViewModel = useMemo<CreateUser>(
    () => ({
      email: "",
      role: props.availableRoles[0],
      organizationId: props.organizationId,
    }),
    [props.organizationId, props.availableRoles]
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

  const handleRoleChange = (event: SelectChangeEvent<Role>) => {
    const role = event.target.value;
    if (typeof role === "string") {
      setUserViewModel({
        ...userViewModel,
        role: adaptRole(role),
      });
    }
  };

  const formDisable = formLoading;

  const displayRoleChoice = props.availableRoles.length > 1;

  return (
    <Dialog open={props.open} onClose={handleClose} fullWidth maxWidth="sm">
      <DelayedLinearProgress loading={formLoading} />

      <DialogTitle>{props.title}</DialogTitle>
      <DialogContent>
        {props.children}
        <TextField
          sx={{ mb: 4 }}
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

        {displayRoleChoice && (
          <>
            <InputLabel id="select-role-label">Role</InputLabel>
            <Select
              labelId="select-role-label"
              id="select-role-select"
              value={userViewModel.role}
              variant="standard"
              label="Role"
              onChange={handleRoleChange}
            >
              {props.availableRoles.map((role, index) => (
                <MenuItem key={index} value={role as string}>
                  {role.toLowerCase()}
                </MenuItem>
              ))}
            </Select>
          </>
        )}
      </DialogContent>

      <DialogActions>
        <Button onClick={handleClose}>Cancel</Button>

        <Button onClick={handleValidate} disabled={!valid || formDisable}>
          Create
        </Button>
      </DialogActions>
    </Dialog>
  );
}
