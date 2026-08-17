import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  DropdownMenu,
  Select,
  ThemeProvider,
  ThemeToggle,
  useTheme,
} from "@gantry/design-system";
import { useState } from "react";

function ThemeConsumer() {
  const { mode, theme, toggleTheme, setMode } = useTheme();
  return (
    <div>
      <span data-testid="mode-display">{mode}</span>
      <span data-testid="theme-display">{theme}</span>
      <button type="button" onClick={toggleTheme}>
        Toggle
      </button>
      <button type="button" onClick={() => setMode("light")}>
        Set Light
      </button>
      <button type="button" onClick={() => setMode("dark")}>
        Set Dark
      </button>
      <ThemeToggle variant="segmented" />
      <ThemeToggle variant="button" />
      <ThemeToggle variant="icon" />
    </div>
  );
}

function SelectWrapper() {
  const [val, setVal] = useState("opt1");
  const options = [
    { value: "opt1", label: "First Option" },
    { value: "opt2", label: "Second Option" },
  ];
  return (
    <Select
      label="Test Select"
      options={options}
      value={val}
      onChange={setVal}
    />
  );
}

describe("Theme System & Select Component", () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute("data-theme");
    document.documentElement.className = "";
  });

  it("initializes theme provider and applies theme attributes to document", async () => {
    render(
      <ThemeProvider defaultMode="dark">
        <ThemeConsumer />
      </ThemeProvider>,
    );

    expect(screen.getByTestId("mode-display")).toHaveTextContent("dark");
    expect(screen.getByTestId("theme-display")).toHaveTextContent("dark");
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    expect(document.documentElement).toHaveClass("dark");
  });

  it("switches between light and dark themes and updates localStorage", async () => {
    const user = userEvent.setup();
    render(
      <ThemeProvider defaultMode="dark">
        <ThemeConsumer />
      </ThemeProvider>,
    );

    const setLightBtn = screen.getByRole("button", { name: "Set Light" });
    await user.click(setLightBtn);

    expect(screen.getByTestId("mode-display")).toHaveTextContent("light");
    expect(screen.getByTestId("theme-display")).toHaveTextContent("light");
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
    expect(document.documentElement).toHaveClass("light");
    expect(localStorage.getItem("gantry_theme_mode")).toBe("light");
  });

  it("renders custom Select, opens dropdown, and selects an option", async () => {
    const user = userEvent.setup();
    render(<SelectWrapper />);

    const trigger = screen.getByRole("combobox", { name: "Test Select" });
    expect(trigger).toBeInTheDocument();
    expect(trigger).toHaveTextContent("First Option");

    // Click trigger to open dropdown
    await user.click(trigger);
    const listbox = screen.getByRole("listbox");
    expect(listbox).toBeInTheDocument();

    // Select second option inside the floating listbox
    const option2 = within(listbox).getByRole("option", {
      name: "Second Option",
    });
    await user.click(option2);

    expect(trigger).toHaveTextContent("Second Option");
  });

  it("renders DropdownMenu and handles item clicks", async () => {
    const user = userEvent.setup();
    const actionMock = vi.fn();

    render(
      <DropdownMenu
        trigger={<button type="button">Menu</button>}
        items={[
          { id: "item1", label: "Action 1", onClick: actionMock },
          "divider",
          { id: "item2", label: "Action 2" },
        ]}
      />,
    );

    const trigger = screen.getByRole("button", { name: "Menu" });
    await user.click(trigger);

    const menuItem = screen.getByRole("menuitem", { name: "Action 1" });
    await user.click(menuItem);

    expect(actionMock).toHaveBeenCalledTimes(1);
  });
});
