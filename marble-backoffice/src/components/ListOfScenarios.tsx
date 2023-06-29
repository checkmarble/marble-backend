import { Scenario } from "@/models";
import { DataGrid, GridRowsProp, GridColDef } from "@mui/x-data-grid";

import CustomGridCell from "./CustomGridCell";

interface ListOfScenariosProps {
  scenarios: Scenario[];
}

function ListOfScenarios(props: ListOfScenariosProps) {
  const scenarios = props.scenarios;
  if (scenarios === null || scenarios.length === 0) {
    return <>No scenarios</>;
  }

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

export default ListOfScenarios;
