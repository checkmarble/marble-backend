import { useState, PropsWithChildren } from "react";
import Button from "@mui/material/Button";
import TextField from "@mui/material/TextField";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";

interface FormDialogProps {
  open: boolean;
  title: string;
  message: string;
  inputLabel: string;
  okTitle: string;
  setDialogOpen: (open: boolean)=> void;
  onValidate: (text: string) => void;
}
export default function FormDialog(props: PropsWithChildren<FormDialogProps>) {
  const [text, setText] = useState("");

  const handleClose = () => {
    props.setDialogOpen(false)
  };

  const handleValidate = () => {
    setText("")
    props.setDialogOpen(false)
    props.onValidate(text);
  };

  const valid = !!text.trim()

  const handleTextChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setText(event.target.value);
  };

  return (
    <div>
      <Dialog open={props.open} onClose={handleClose}>
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
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={handleClose}>Cancel</Button>
          <Button onClick={handleValidate} disabled={!valid} >{props.okTitle}</Button>
        </DialogActions>
      </Dialog>
    </div>
  );
}
