import Box from "@mui/material/Box";
import IconButton from "@mui/material/IconButton";
import { GridCell, GridCellProps } from "@mui/x-data-grid";
import { useState } from "react";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";

export default function CustomGridCell(props: GridCellProps) {
  // Extract editCellState from props to avoid warning
  const { editCellState, ...other } = props;

  const [hover, setHover] = useState<boolean>(false);
  const displayHover = hover && props.column.cellClassName != "noHover";

  const handleCopyToClipboardclick = () => {
    navigator.clipboard.writeText(JSON.stringify(props.value));
  };

  return (
    <Box
      sx={{
        position: "relative",
      }}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
    >
      <GridCell {...other} />

      {displayHover && (
        <IconButton
          sx={{
            position: "absolute",
            top: "50%",
            transform: "translateY(-50%)",
            right: "0",
          }}
          onClick={handleCopyToClipboardclick}
        >
          <ContentCopyIcon />
        </IconButton>
      )}
    </Box>
  );
}
