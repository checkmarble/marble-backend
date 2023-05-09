import { useCallback, useState } from "react";
import IconButton from "@mui/material/IconButton";
import AppBar from "@mui/material/AppBar";
import Toolbar from "@mui/material/Toolbar";
import Box from "@mui/material/Box";
import Typography from "@mui/material/Typography";
import Button from "@mui/material/Button";
import Drawer from "@mui/material/Drawer";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemIcon from "@mui/material/ListItemIcon";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemText from "@mui/material/ListItemText";
import MenuIcon from "@mui/icons-material/Menu";
import InboxIcon from "@mui/icons-material/MoveToInbox";
import MailIcon from "@mui/icons-material/Mail";

interface Page {
  title: string;
  link: string;
}

const APP_BAR_PAGES: ReadonlyArray<Page> = [
  {
    title: "Home",
    link: "/home",
  },
  {
    title: "Organizations",
    link: "/organizations",
  },
] as const;

function BackOfficeAppBar() {
  const handleNavigate = useCallback((link: string) => {
    console.log(`Navigation not implemented: ${link}`);
  }, []);

  const [drawerOpen, setDrawerOpen] = useState<boolean>(false);

  const toggleDrawer = useCallback(
    () => setDrawerOpen(!drawerOpen),
    [drawerOpen]
  );
  const closeDrawer = useCallback(() => setDrawerOpen(false), []);

  return (
    <AppBar position="static">
      <Toolbar disableGutters>
        {/* Desktop App bar */}
        <Typography
          variant="h6"
          noWrap
          component="a"
          href="/"
          sx={{
            display: { xs: "none", md: "flex" },
            mr: 2,
            ml: 2,
            color: "inherit",
            textDecoration: "none",
          }}
        >
          Marble BackOffice
        </Typography>
        <Box sx={{ display: { xs: "none", md: "flex" }, flexgrow: 1 }}>
          {APP_BAR_PAGES.map((page: Page, index) => (
            <Button
              key={index}
              onClick={() => handleNavigate(page.link)}
              sx={{
                my: 2,
                color: "white",
                display: "block",
              }}
            >
              {page.title}
            </Button>
          ))}
        </Box>

        {/* Mobile App Bar */}
        <Box sx={{ display: { xs: "flex", md: "none" }, flexGrow: 1 }}>
          <IconButton
            size="large"
            aria-label="Open drawer"
            color="inherit"
            onClick={toggleDrawer}
          >
            <MenuIcon />
          </IconButton>
          <Typography
            variant="h6"
            noWrap
            component="a"
            href="/"
            sx={{
              display: { xs: "box", md: "none" },
              mr: 2,
              mt: 1,
              color: "inherit",
              textDecoration: "none",
            }}
          >
            Marble BackOffice
          </Typography>
          <Drawer anchor="left" open={drawerOpen} onClose={closeDrawer}>
            <Box
              sx={{
                width: 250,
              }}
              role="presentation"
              onClick={closeDrawer}
              onKeyDown={closeDrawer}
            >
              <List>
                {APP_BAR_PAGES.map((page: Page, index) => (
                  <ListItem key={index} disablePadding>
                    <ListItemButton onClick={() => handleNavigate(page.link)}>
                      <ListItemIcon>
                        {index % 2 === 0 ? <InboxIcon /> : <MailIcon />}
                      </ListItemIcon>
                      <ListItemText primary={page.title} />
                    </ListItemButton>
                  </ListItem>
                ))}
              </List>
            </Box>
          </Drawer>
        </Box>
      </Toolbar>
    </AppBar>
  );
}

export default BackOfficeAppBar;
