import { Organization } from "@/models";
import {
  DataGrid,
  GridRowsProp,
  GridColDef,
  GridRenderCellParams,
} from "@mui/x-data-grid";
import Button from "@mui/material/Button";
import NorthEastIcon from "@mui/icons-material/NorthEast";
import GridCellWithTooltip from "./CustomGridCell";

interface ListOfOrganizationsProps {
  organizations: Organization[] | null;
  onOrganizationDetailClick?: (organizationId: string) => void;
}

function ListOfOrganizations(props: ListOfOrganizationsProps) {
  const organizations = props.organizations;
  if (organizations == null) {
    return <>No organizations</>;
  }

  // Enrich the 'organization' to build a 'row'
  const rows: GridRowsProp = organizations.map((organization) => ({
    id: organization.organizationId, // needed for datagrid
    actions: organization.organizationId, // needed to build a fake 'actions' column
    onDetailsClick: props.onOrganizationDetailClick
      ? () => props.onOrganizationDetailClick!(organization.organizationId)
      : null, // build user-specific actions using the props

    ...organization, // keep all org data intact
  }));

  const columns: GridColDef[] = [
    { field: "name", headerName: "name", flex: 1 },
    { field: "organizationId", headerName: "ID", flex: 1 },
  ];

  // Actions, only add column if there actually are actions
  if (props.onOrganizationDetailClick) {
    columns.push({
      field: "actions",
      headerName: "actions",
      flex: 0.5,
      renderCell: ListOfOrganizationsActionsCell,
      cellClassName: "noHover",
    });
  }

  return (
    <DataGrid
      rows={rows}
      columns={columns}
      slots={{ cell: GridCellWithTooltip }}
      disableRowSelectionOnClick
    />
  );
}

// see https://github.com/mui/mui-x/blob/master/packages/grid/x-data-grid-generator/src/renderer/renderRating.tsx
// for reference
function ListOfOrganizationsActionsCell(
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

export default ListOfOrganizations;
