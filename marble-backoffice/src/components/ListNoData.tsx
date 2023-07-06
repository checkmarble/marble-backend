import { Stack, Typography } from "@mui/material";
import DataObjectIcon from "@mui/icons-material/DataObject";

export default function ListNoData() {
  return (
    <Stack
      direction="row"
      justifyContent="flex-start"
      alignItems="center"
      spacing={2}
      m={2}
    >
      <DataObjectIcon color="secondary" />
      <Typography>No data to display</Typography>
    </Stack>
  );
}
