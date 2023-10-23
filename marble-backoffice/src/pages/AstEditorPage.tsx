// import DelayedLinearProgress from "@/components/DelayedLinearProgress";
// import Container from "@mui/material/Container";
// import TextareaAutosize from "@mui/material/TextareaAutosize";
// import Typography from "@mui/material/Typography";
// import { useLoading } from "@/hooks/Loading";
// import services from "@/injectServices";
// import { useAstEditor, useSingleScenario } from "@/services";
// import { useNavigate, useParams } from "react-router-dom";
// import Alert from "@mui/material/Alert";
// import Button from "@mui/material/Button";
// import { PageLink } from "@/models";

// export default function AstEditorPage() {
//   const { scenarioId, iterationId, ruleId } = useParams();

//   if (!scenarioId) {
//     throw Error("scenarioId is required");
//   }

//   if (!iterationId) {
//     throw Error("iterationId is required");
//   }

//   const navigate = useNavigate();

//   const [pageLoading, pageLoadingDispatcher] = useLoading();

//   const { scenario, iteration } = useSingleScenario({
//     service: services().scenarioService,
//     loadingDispatcher: pageLoadingDispatcher,
//     scenarioId,
//     iterationId,
//   });

//   const { astText, setAstText, errorMessages, saveTriggerOrRule } =
//     useAstEditor(
//       services().astEditorService,
//       pageLoadingDispatcher,
//       scenario,
//       iteration,
//       ruleId ?? null
//     );

//   const handleSaveTriggerOrRule = async () => {
//     if (await saveTriggerOrRule()) {
//       navigate(PageLink.scenarioDetailsPage(scenarioId, iterationId));
//     }
//   };

//   return (
//     <>
//       <DelayedLinearProgress loading={pageLoading} />
//       <Container
//         sx={{
//           mb: 4,
//         }}
//       >
//         <Typography variant="h4">AstEditor</Typography>

//         {scenarioId && (
//           <Typography variant="body1">scenarioId: {scenarioId}</Typography>
//         )}
//         <Typography>
//           {ruleId !== undefined ? `Edit rule: ${ruleId}` : "edit trigger"}
//         </Typography>
//         {astText && (
//           <TextareaAutosize
//             style={{ width: "100%" }}
//             value={astText}
//             onChange={(e) => setAstText(e.target.value)}
//           />
//         )}
//         {errorMessages.map((error, i) => (
//           <Alert key={i} severity="error">
//             {error}
//           </Alert>
//         ))}
//         <Button disabled={pageLoading} onClick={handleSaveTriggerOrRule}>
//           Save
//         </Button>
//       </Container>
//     </>
//   );
// }
