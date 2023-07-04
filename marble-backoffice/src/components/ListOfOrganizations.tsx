import { Organization } from "@/models";
import { DataGrid } from "@mui/x-data-grid/DataGrid";
import type { GridRowsProp, GridColDef, GridRowParams } from "@mui/x-data-grid";
import { GridActionsCellItem } from "@mui/x-data-grid";
import NorthEastIcon from "@mui/icons-material/NorthEast";

import GridCellWithHover from "./GridCellWithHover";
import ListNoData from "./ListNoData";

interface ListOfOrganizationsProps {
  organizations: Organization[] | null;
  onOrganizationDetailClick: (organizationId: string) => void;
}

export default function ListOfOrganizations(props: ListOfOrganizationsProps) {
  const organizations = props.organizations;

  if (organizations === null) {
    return;
  }

  // empty state
  if (organizations.length === 0) {
    return <ListNoData />;
  }

  // Enrich the 'organization' to build a 'row'
  const rows: GridRowsProp = organizations.map((organization) => ({
    id: organization.organizationId, // needed for datagrid
    ...organization, // keep all org data intact
  }));

  const columns: GridColDef[] = [
    {
      field: "name",
      headerName: "name",
      flex: 1,
      renderCell: GridCellWithHover,
    },
    {
      field: "organizationId",
      headerName: "ID",
      flex: 1,
      renderCell: GridCellWithHover,
    },
    {
      field: "actions",
      type: "actions",
      getActions: (params: GridRowParams) => [
        <GridActionsCellItem
          icon={<NorthEastIcon />}
          onClick={() =>
            props.onOrganizationDetailClick(params.row.organizationId)
          }
          label="Details"
        />,
      ],
    },
  ];

  return <DataGrid rows={rows} columns={columns} disableRowSelectionOnClick />;
}
