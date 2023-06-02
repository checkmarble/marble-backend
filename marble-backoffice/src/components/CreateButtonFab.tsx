import Fab from "@mui/material/Fab";
import AddIcon from "@mui/icons-material/Add";

interface CreateButtonFabProps {
  title: string;
  onClick: () => void;
}

export default function CreateButtonFab(props: CreateButtonFabProps) {
  return (
    <Fab
      sx={{
        position: "absolute",
        top: "10px",
        right: "50px",
        paddingRight: "20px",
      }}
      color="primary"
      size="small"
      variant="extended"
      aria-label="add"
      onClick={props.onClick}
    >
      <AddIcon sx={{ mr: 1 }} />
      {props.title}
    </Fab>
  );
}
