import Button from "@mui/material/Button";
import Dialog, { DialogProps } from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";

interface AlertDialogProps extends Omit<DialogProps, "onClose"> {
  handleClose: () => void;
  handleValidate: () => void;
  children: React.ReactNode;
  title: string;
}

export default function AlertDialog({
  handleClose,
  handleValidate,
  children,
  title,
  ...otherProps
}: AlertDialogProps) {
  return (
    <Dialog {...otherProps} onClose={handleClose}>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent role="alertdialog">
        {children}
        <DialogActions>
          <Button onClick={handleClose}>Cancel</Button>

          <Button onClick={handleValidate} color="error">
            Validate
          </Button>
        </DialogActions>
      </DialogContent>
    </Dialog>
  );
}
