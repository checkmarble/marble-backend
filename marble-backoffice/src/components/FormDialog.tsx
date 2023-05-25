import { useState, PropsWithChildren } from "react";
import Button from "@mui/material/Button";
import TextField from "@mui/material/TextField";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";
import { showLoader, useLoading } from "@/hooks/Loading";
import DelayedLinearProgress from "./DelayedLinearProgress";

interface FormDialogProps {
  open: boolean;
  title: string;
  message: string;
  inputLabel: string;
  okTitle: string;
  setDialogOpen: (open: boolean) => void;
  onValidate: (text: string) => Promise<void>;
}
export default function FormDialog(props: PropsWithChildren<FormDialogProps>) {
  const [text, setText] = useState("");
  const [formLoading, formLoadingDispatcher] = useLoading();

  const handleClose = () => {
    props.setDialogOpen(false);
  };

  const handleValidate = async () => {
    await showLoader(formLoadingDispatcher, props.onValidate(text));
    setText("");
    props.setDialogOpen(false);
  };

  const valid = !!text.trim();

  const handleTextChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setText(event.target.value);
  };

  const formDisable = formLoading;

  return (
    <div>
      <Dialog open={props.open} onClose={handleClose}>
        <DelayedLinearProgress loading={formLoading} />

        <DialogTitle>{props.title}</DialogTitle>
        <DialogContent>
          <DialogContentText>{props.message}</DialogContentText>
          {props.children}
          <TextField
            autoFocus
            margin="dense"
            id="name"
            label={props.inputLabel}
            // type="email"
            fullWidth
            variant="standard"
            value={text}
            onChange={handleTextChange}
            disabled={formDisable}
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={handleClose}>Cancel</Button>

          <Button onClick={handleValidate} disabled={!valid || formDisable}>
            {props.okTitle}
          </Button>
        </DialogActions>
      </Dialog>
    </div>
  );
}
