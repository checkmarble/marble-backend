import { User } from "@/models";
import {
  DataGrid,
  GridRowsProp,
  GridColDef,
  GridRenderCellParams,
} from "@mui/x-data-grid";
import Button from "@mui/material/Button";
import NorthEastIcon from "@mui/icons-material/NorthEast";

import CustomGridCell from "./CustomGridCell";

interface ListOfUsersProps {
  users: User[] | null;
  onUserDetailClick?: (userId: string) => void;
}

function ListOfUsers(props: ListOfUsersProps) {
  const users = props.users;
  if (users == null || users.length == 0) {
    return <>No users</>;
  }

  // Enrich the 'user' to build a 'row'
  const rows: GridRowsProp = users.map((user) => ({
    id: user.userId, // needed for datagrid
    actions: user.userId, // needed to build a fake 'actions' column
    onDetailsClick: props.onUserDetailClick
      ? () => props.onUserDetailClick!(user.userId)
      : null, // build user-specific actions using the props

    ...user, // keep all user data intact
  }));

  const columns: GridColDef[] = [
    { field: "email", headerName: "email", flex: 1 },
    { field: "role", headerName: "role", flex: 1 },
    { field: "userId", headerName: "ID", flex: 1 },
  ];

  // Actions, only add column if there actually are actions
  if (props.onUserDetailClick) {
    columns.push({
      field: "actions",
      headerName: "actions",
      flex: 0.5,
      renderCell: ListOfUsersActionsCell,
      cellClassName: "noHover",
    });
  }

  return (
    <DataGrid
      rows={rows}
      columns={columns}
      slots={{ cell: CustomGridCell }}
      disableRowSelectionOnClick
    />
  );
}

// see https://github.com/mui/mui-x/blob/master/packages/grid/x-data-grid-generator/src/renderer/renderRating.tsx
// for reference
function ListOfUsersActionsCell(
  params: GridRenderCellParams<any, number, any>
) {
  if (params.value == null) {
    console.warn("ListOfUsersActionsCell : params.value == null");
    return null;
  }

  if (params.row.onDetailsClick == null) {
    console.warn("ListOfUsersActionsCell : params.row.onDetailsClick == null");
    return null;
  }

  return (
    <>
      <Button
        size="small"
        variant="outlined"
        startIcon={<NorthEastIcon fontSize="small" />}
        onClick={params.row.onDetailsClick}
      >
        Details
      </Button>
    </>
  );
}

export default ListOfUsers;
