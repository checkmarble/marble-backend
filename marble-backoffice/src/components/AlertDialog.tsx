import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogProps,
  DialogTitle,
} from "@mui/material";

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
