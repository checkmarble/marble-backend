import { Scenario } from "@/models";
import { DataGrid } from "@mui/x-data-grid/DataGrid/DataGrid";
import type { GridRowsProp, GridColDef } from "@mui/x-data-grid/models";
import GridCellWithHover from "./GridCellWithHover";
import ListNoData from "./ListNoData";
import { GridActionsCellItem } from "@mui/x-data-grid/components/cell/GridActionsCellItem";
import NorthEastIcon from "@mui/icons-material/NorthEast";

interface ListOfScenariosProps {
  scenarios: Scenario[] | null;
  onScenarioDetailClick: (scenarioId: string) => void;
}

function ListOfScenarios(props: ListOfScenariosProps) {
  const scenarios = props.scenarios;

  if (scenarios === null) {
    return;
  }

  // empty state
  if (scenarios.length === 0) {
    return <ListNoData />;
  }

  // Enrich the 'scenarios' to build a 'row'
  const rows: GridRowsProp<Scenario> = scenarios.map((scenario) => ({
    id: scenario.scenarioId, // needed for datagrid
    ...scenario, // keep all user data intact
  }));

  // Static columns
  const columns: GridColDef<Scenario>[] = [
    {
      field: "name",
      headerName: "name",
      flex: 1,
      renderCell: GridCellWithHover,
    },
    {
      field: "description",
      headerName: "description",
      flex: 1,
      renderCell: GridCellWithHover,
    },
    {
      field: "triggerObjectType",
      headerName: "triggerObjectType",
      flex: 1,
      renderCell: GridCellWithHover,
    },
    {
      field: "createdAt",
      headerName: "created at",
      type: "dateTime",
      flex: 1,
      renderCell: GridCellWithHover,
    },
    {
      field: "scenarioId",
      headerName: "ID",
      flex: 1,
      renderCell: GridCellWithHover,
    },
    {
      field: "actions",
      type: "actions",
      getActions: (params) => [
        // only add the item if(props.onUserDetailClick) : see https://stackoverflow.com/questions/44908159/how-to-define-an-array-with-conditional-elements
        ...(props.onScenarioDetailClick
          ? [
              <GridActionsCellItem
                icon={<NorthEastIcon />}
                onClick={() =>
                  props.onScenarioDetailClick(params.row.scenarioId)
                }
                label="Details"
              />,
            ]
          : []),
      ],
    },
  ];

  return (
    <>
      <DataGrid rows={rows} columns={columns} disableRowSelectionOnClick />
    </>
  );
}
export default ListOfScenarios;
