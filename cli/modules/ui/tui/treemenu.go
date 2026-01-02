package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TreeMenuItem represents an item in the tree menu
type TreeMenuItem struct {
	ID       string
	Label    string
	Icon     string         // emoji or character
	Children []TreeMenuItem
	Data     interface{}    // arbitrary data attached to the item
	Count    int            // optional count to display (e.g., number of children)
}

// TreeMenu is a navigable tree menu with drill-down support
type TreeMenu struct {
	items         []TreeMenuItem
	drillDownPath []string // IDs of items we've drilled into
	selectedIndex int      // currently selected index in current view
	focused       bool
	title         string

	// Search
	searchQuery  string
	searchActive bool

	// Styling
	width  int
	height int
}

// NewTreeMenu creates a new tree menu
func NewTreeMenu(items []TreeMenuItem) *TreeMenu {
	return &TreeMenu{
		items:         items,
		drillDownPath: []string{},
		selectedIndex: 0,
		title:         "Menu",
	}
}

// SetTitle sets the menu title
func (tm *TreeMenu) SetTitle(title string) {
	tm.title = title
}

// SetItems updates the menu items
func (tm *TreeMenu) SetItems(items []TreeMenuItem) {
	tm.items = items
	// Reset selection if out of bounds
	currentItems := tm.visibleItems()
	if tm.selectedIndex >= len(currentItems) {
		tm.selectedIndex = max(0, len(currentItems)-1)
	}
}

// SetFocused sets the focus state
func (tm *TreeMenu) SetFocused(focused bool) {
	tm.focused = focused
}

// SetSize sets the dimensions
func (tm *TreeMenu) SetSize(width, height int) {
	tm.width = width
	tm.height = height
}

// SetSearchQuery sets the search filter
func (tm *TreeMenu) SetSearchQuery(query string) {
	tm.searchQuery = query
	tm.selectedIndex = 0 // Reset selection when search changes
}

// SearchQuery returns the current search query
func (tm *TreeMenu) SearchQuery() string {
	return tm.searchQuery
}

// SetSearchActive sets whether search input is active
func (tm *TreeMenu) SetSearchActive(active bool) {
	tm.searchActive = active
}

// IsSearchActive returns whether search input is active
func (tm *TreeMenu) IsSearchActive() bool {
	return tm.searchActive
}

// currentLevelItems returns the items at the current drill-down level (unfiltered)
func (tm *TreeMenu) currentLevelItems() []TreeMenuItem {
	items := tm.items
	for _, id := range tm.drillDownPath {
		for _, item := range items {
			if item.ID == id {
				items = item.Children
				break
			}
		}
	}
	return items
}

// filterItems recursively filters items based on search query
func (tm *TreeMenu) filterItems(items []TreeMenuItem, query string) []TreeMenuItem {
	if query == "" {
		return items
	}

	query = strings.ToLower(query)
	var result []TreeMenuItem

	for _, item := range items {
		// Check if this item matches
		if strings.Contains(strings.ToLower(item.Label), query) {
			result = append(result, item)
			continue
		}

		// Check if any children match (recursively)
		if len(item.Children) > 0 {
			filteredChildren := tm.filterItems(item.Children, query)
			if len(filteredChildren) > 0 {
				// Include this item with filtered children
				itemCopy := item
				itemCopy.Children = filteredChildren
				result = append(result, itemCopy)
			}
		}
	}

	return result
}

// visibleItems returns the items currently visible (filtered if searching)
func (tm *TreeMenu) visibleItems() []TreeMenuItem {
	items := tm.currentLevelItems()
	if tm.searchQuery != "" {
		items = tm.filterItems(items, tm.searchQuery)
	}
	return items
}

// currentParent returns the parent item if we're drilled down, nil otherwise
func (tm *TreeMenu) currentParent() *TreeMenuItem {
	if len(tm.drillDownPath) == 0 {
		return nil
	}

	items := tm.items
	var parent *TreeMenuItem
	for _, id := range tm.drillDownPath {
		for i := range items {
			if items[i].ID == id {
				parent = &items[i]
				items = items[i].Children
				break
			}
		}
	}
	return parent
}

// SelectedItem returns the currently selected item, or nil if none
func (tm *TreeMenu) SelectedItem() *TreeMenuItem {
	items := tm.visibleItems()
	if tm.selectedIndex < 0 || tm.selectedIndex >= len(items) {
		return nil
	}
	return &items[tm.selectedIndex]
}

// SelectedIndex returns the current selection index
func (tm *TreeMenu) SelectedIndex() int {
	return tm.selectedIndex
}

// SetSelectedIndex sets the selection index
func (tm *TreeMenu) SetSelectedIndex(index int) {
	items := tm.visibleItems()
	if index < 0 {
		index = 0
	}
	if index >= len(items) {
		index = len(items) - 1
	}
	tm.selectedIndex = index
}

// IsAtRoot returns true if we're at the root level
func (tm *TreeMenu) IsAtRoot() bool {
	return len(tm.drillDownPath) == 0
}

// DrillDownPath returns the current drill-down path
func (tm *TreeMenu) DrillDownPath() []string {
	return tm.drillDownPath
}

// MoveUp moves selection up
func (tm *TreeMenu) MoveUp() {
	if tm.selectedIndex > 0 {
		tm.selectedIndex--
	}
}

// MoveDown moves selection down
func (tm *TreeMenu) MoveDown() {
	items := tm.visibleItems()
	if tm.selectedIndex < len(items)-1 {
		tm.selectedIndex++
	}
}

// DrillDown enters the selected item if it has children
// Returns true if drill-down happened
func (tm *TreeMenu) DrillDown() bool {
	item := tm.SelectedItem()
	if item == nil || len(item.Children) == 0 {
		return false
	}
	tm.drillDownPath = append(tm.drillDownPath, item.ID)
	tm.selectedIndex = 0
	tm.searchQuery = "" // Clear search when drilling down
	return true
}

// DrillUp goes back to parent level
// Returns true if drill-up happened
func (tm *TreeMenu) DrillUp() bool {
	if len(tm.drillDownPath) == 0 {
		return false
	}
	tm.drillDownPath = tm.drillDownPath[:len(tm.drillDownPath)-1]
	tm.selectedIndex = 0
	tm.searchQuery = "" // Clear search when drilling up
	return true
}

// Select activates the current item (drill-down if has children, otherwise return the item)
// Returns the selected leaf item, or nil if drilled down
func (tm *TreeMenu) Select() *TreeMenuItem {
	item := tm.SelectedItem()
	if item == nil {
		return nil
	}
	if len(item.Children) > 0 {
		tm.DrillDown()
		return nil
	}
	return item
}

// ClearSearch clears the search query
func (tm *TreeMenu) ClearSearch() {
	tm.searchQuery = ""
	tm.selectedIndex = 0
}

// CalcWidth calculates the optimal width based on item labels
func (tm *TreeMenu) CalcWidth() int {
	const minWidth = 30
	const maxWidth = 60
	const padding = 14 // icons, cursor, borders, count suffix

	maxLen := 0

	// Check all items recursively
	var checkItems func(items []TreeMenuItem)
	checkItems = func(items []TreeMenuItem) {
		for _, item := range items {
			labelLen := len(item.Label)
			// Add space for count if present
			if item.Count > 0 {
				labelLen += 5 // " (XX)"
			}
			if labelLen > maxLen {
				maxLen = labelLen
			}
			if len(item.Children) > 0 {
				checkItems(item.Children)
			}
		}
	}
	checkItems(tm.items)

	width := maxLen + padding
	if width < minWidth {
		return minWidth
	}
	if width > maxWidth {
		return maxWidth
	}
	return width
}

// Render renders the tree menu
func (tm *TreeMenu) Render() string {
	width := tm.width
	if width == 0 {
		width = tm.CalcWidth()
	}

	items := tm.visibleItems()
	parent := tm.currentParent()
	innerWidth := width - 4 // Account for border

	var lines []string

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)

	if parent != nil {
		// Show back button with parent name
		backLine := "← " + truncate(parent.Label, innerWidth-4)
		lines = append(lines, headerStyle.Render(backLine))
	} else {
		lines = append(lines, headerStyle.Render(tm.title))
	}

	// Separator
	lines = append(lines, lipgloss.NewStyle().
		Foreground(ColorBorder).
		Render(strings.Repeat("─", innerWidth)))

	// Search bar (if active or has query)
	if tm.searchActive || tm.searchQuery != "" {
		searchStyle := lipgloss.NewStyle().Foreground(ColorMuted)
		searchIcon := "🔍 "
		searchText := tm.searchQuery
		if tm.searchActive {
			searchText += "█"
		}
		if searchText == "" && !tm.searchActive {
			searchText = "type to search..."
		}
		searchLine := searchIcon + searchText
		lines = append(lines, searchStyle.Render(truncate(searchLine, innerWidth)))
		lines = append(lines, lipgloss.NewStyle().
			Foreground(ColorBorder).
			Render(strings.Repeat("─", innerWidth)))
	}

	// Items
	if len(items) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(ColorMuted)
		if tm.searchQuery != "" {
			lines = append(lines, emptyStyle.Render("No matches"))
		} else {
			lines = append(lines, emptyStyle.Render("No items"))
		}
	} else {
		for i, item := range items {
			isSelected := i == tm.selectedIndex
			hasChildren := len(item.Children) > 0

			// Build the line
			icon := item.Icon
			if icon == "" {
				if hasChildren {
					icon = "📁"
				} else {
					icon = "○"
				}
			}

			// Cursor
			cursor := "  "
			if isSelected && tm.focused {
				cursor = "> "
			}

			// Label with optional count
			label := item.Label
			countSuffix := ""
			if item.Count > 0 || (hasChildren && len(item.Children) > 0) {
				count := item.Count
				if count == 0 {
					count = len(item.Children)
				}
				countSuffix = lipgloss.NewStyle().Foreground(ColorMuted).Render(
					" (" + itoa(count) + ")",
				)
			}

			// Arrow for items with children
			arrow := ""
			if hasChildren {
				arrow = " →"
			}

			// Calculate available space for label
			// cursor(2) + icon(2) + space(1) + label + count + arrow
			availableForLabel := innerWidth - 2 - 3 - len(countSuffix) - len(arrow)
			if availableForLabel < 10 {
				availableForLabel = 10
			}

			line := cursor + icon + " " + truncate(label, availableForLabel) + countSuffix + arrow

			// Apply style
			var style lipgloss.Style
			if isSelected && tm.focused {
				style = lipgloss.NewStyle().
					Bold(true).
					Background(ColorBgAlt).
					Foreground(ColorText).
					Width(innerWidth)
			} else if isSelected {
				style = lipgloss.NewStyle().
					Foreground(ColorPrimary).
					Width(innerWidth)
			} else {
				style = lipgloss.NewStyle().
					Foreground(ColorText).
					Width(innerWidth)
			}

			lines = append(lines, style.Render(line))
		}
	}

	// Spacer
	lines = append(lines, "")

	// Hints
	hintStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	if tm.searchActive {
		lines = append(lines, hintStyle.Render("Esc:cancel /clear"))
	} else if parent != nil {
		lines = append(lines, hintStyle.Render("←/Esc:back /:search"))
	} else {
		item := tm.SelectedItem()
		if item != nil && len(item.Children) > 0 {
			lines = append(lines, hintStyle.Render("↑↓:nav →/Enter:open /:search"))
		} else {
			lines = append(lines, hintStyle.Render("↑↓:nav Enter:select /:search"))
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	// Border
	var style lipgloss.Style
	if tm.focused {
		style = FocusedBorderStyle
	} else {
		style = UnfocusedBorderStyle
	}

	return style.
		Width(width).
		Height(tm.height).
		Render(content)
}

// itoa converts int to string (simple helper to avoid fmt import)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	negative := n < 0
	if negative {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
