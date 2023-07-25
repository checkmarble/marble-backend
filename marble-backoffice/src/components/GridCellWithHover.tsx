import Box from "@mui/material/Box";
import IconButton from "@mui/material/IconButton";
import useTheme from "@mui/material/styles/useTheme";
import { GridRenderCellParams } from "@mui/x-data-grid";
import { useState } from "react";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";

export default function GridCellWithControls(
  params: GridRenderCellParams<any, number, any>
) {
  const valueToDisplay = params.formattedValue
    ? params.formattedValue
    : params.value
    ? params.value
    : null;

  if (typeof valueToDisplay === "object") {
    console.warn(
      "Value to display is an object: there should probably be a column type or valueGetter on that column"
    );
  }

  const [hover, setHover] = useState<boolean>(false);

  const handleCopyToClipboardclick = () => {
    navigator.clipboard.writeText(JSON.stringify(valueToDisplay));
  };

  const theme = useTheme();

  return (
    <Box
      sx={{
        // needed to position hover button
        position: "relative",

        width: "100%",
        height: "100%",

        // needed to vertically center the content
        display: "flex",
        alignItems: "center",
      }}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
    >
      <Box
        sx={{
          // display an ellipsis (...) when celles are too narrow
          whiteSpace: "nowrap",
          overflow: "hidden",
          textOverflow: "ellipsis",
        }}
      >
        {valueToDisplay}
      </Box>

      {hover && (
        <IconButton
          sx={{
            position: "absolute",
            top: "50%",
            transform: "translateY(-50%)",
            right: theme.spacing(1),
          }}
          onClick={handleCopyToClipboardclick}
        >
          <ContentCopyIcon />
        </IconButton>
      )}
    </Box>
  );
}
