import { User } from "@/models";
import { DataGrid } from "@mui/x-data-grid/DataGrid/DataGrid";
import type { GridRowsProp, GridColDef, GridRowParams } from "@mui/x-data-grid/models";
import { GridActionsCellItem } from "@mui/x-data-grid/components/cell/GridActionsCellItem";
import NorthEastIcon from "@mui/icons-material/NorthEast";
import ListNoData from "./ListNoData";
import GridCellWithHover from "./GridCellWithHover";

interface ListOfUsersProps {
  users: User[] | null;
  onUserDetailClick: (userId: string) => void;
}

export default function ListOfUsers(props: ListOfUsersProps) {
  const users = props.users;

  if (users === null) {
    return;
  }
  if (users.length === 0) {
    return <ListNoData />;
  }

  // Enrich the 'user' to build a 'row'
  const rows: GridRowsProp = users.map((user) => ({
    id: user.userId, // needed for datagrid
    ...user, // keep all user data intact
  }));

  const columns: GridColDef[] = [
    {
      field: "email",
      headerName: "email",
      flex: 1,
      renderCell: GridCellWithHover,
    },
    {
      field: "role",
      headerName: "role",
      flex: 1,
      renderCell: GridCellWithHover,
    },
    {
      field: "userId",
      headerName: "ID",
      flex: 1,
      renderCell: GridCellWithHover,
    },
    {
      field: "actions",
      type: "actions",
      getActions: (params: GridRowParams) => [
        // only add the item if(props.onUserDetailClick) : see https://stackoverflow.com/questions/44908159/how-to-define-an-array-with-conditional-elements
        ...(props.onUserDetailClick
          ? [
              <GridActionsCellItem
                icon={<NorthEastIcon />}
                onClick={() => props.onUserDetailClick(params.row.userId)}
                label="Details"
              />,
            ]
          : []),
      ],
    },
  ];

  return <DataGrid rows={rows} columns={columns} disableRowSelectionOnClick />;
}
