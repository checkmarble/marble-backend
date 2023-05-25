import Fade from "@mui/material/Fade";
import LinearProgress from "@mui/material/LinearProgress";

interface DelayedLinearProgressPropsProps {
  loading: boolean;
}

export default function DelayedLinearProgress(
  props: DelayedLinearProgressPropsProps
) {
  return (
    <Fade
      in={props.loading}
      style={{
        transitionDelay: props.loading ? "800ms" : "0ms",
      }}
    >
      <LinearProgress color="inherit" />
    </Fade>
  );
}
