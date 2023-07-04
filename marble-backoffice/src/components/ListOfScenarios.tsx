import { PageLink, Scenario } from "@/models";
import { DataGrid } from "@mui/x-data-grid/DataGrid";
import type {
  GridRowsProp,
  GridColDef,
  GridRenderCellParams,
} from "@mui/x-data-grid/models";

import CustomGridCell from "./CustomGridCell";
import Button from "@mui/material/Button";
import NorthEastIcon from "@mui/icons-material/NorthEast";
import { useNavigate } from "react-router-dom";

interface ListOfScenariosProps {
  scenarios: Scenario[];
}

function ListOfScenarios(props: ListOfScenariosProps) {
  const scenarios = props.scenarios;
  //if(scenarios==null){return(<>No scenarios</>)}

  // Enrich the 'scenarios' to build a 'row'
  const rows: GridRowsProp = scenarios.map((scenario) => ({
    id: scenario.scenarioId, // needed for datagrid
    actions: scenario.scenarioId, // needed to build a fake 'actions' column
    ...scenario, // keep all user data intact
  }));

  // Static columns
  const columns: GridColDef[] = [
    { field: "name", headerName: "name", flex: 1 },
    { field: "description", headerName: "description", flex: 1 },
    { field: "triggerObjectType", headerName: "triggerObjectType", flex: 1 },
    { field: "createdAt", headerName: "created at", flex: 1 },
    { field: "scenarioId", headerName: "ID", flex: 1 },
    {
      field: "actions",
      headerName: "actions",
      flex: 1,
      renderCell: ScenarioDetailsActionsCell,
      cellClassName: "noHover",
    },
  ];

  return (
    <>
      <DataGrid
        rows={rows}
        columns={columns}
        slots={{ cell: CustomGridCell }}
        disableRowSelectionOnClick
      />
    </>
  );
}

function ScenarioDetailsActionsCell(params: GridRenderCellParams) {
  const navigate = useNavigate();

  return (
    <>
      <Button
        size="small"
        variant="outlined"
        startIcon={<NorthEastIcon fontSize="small" />}
        onClick={() => navigate(PageLink.scenarioDetailsPage(params.row.id))}
      >
        Details
      </Button>
    </>
  );
}

export default ListOfScenarios;
