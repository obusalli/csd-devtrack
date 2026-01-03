package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TreeMenuItem represents an item in the tree menu
type TreeMenuItem struct {
	ID           string
	Label        string
	Icon         string              // emoji or character (left side)
	IconColor    lipgloss.TerminalColor // optional color for the icon
	TrailingIcon string              // emoji or character (right side, e.g., ⚡ for attached)
	Children     []TreeMenuItem
	Data         interface{}         // arbitrary data attached to the item
	Count        int                 // optional count to display (e.g., number of children)
	IsActive     bool                // whether this item is the currently active one
	Blink        bool                // whether the icon should blink (for deleting state)
	Disabled     bool                // whether this item is disabled (can't be selected)
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

	// Rename
	renameActive bool
	renameText   string

	// Styling
	width           int
	height          int
	rightSidePanel  bool // If true, apply width adjustment to prevent right border cutoff

	// Scrolling
	scrollOffset int // First visible item index

	// Animation
	blinkState bool // Toggles for blinking items
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
	total := tm.TotalVisibleCount()
	if tm.selectedIndex >= total {
		tm.selectedIndex = max(0, total-1)
	}
	// Ensure scroll is valid
	tm.ensureSelectionVisible()
}

// Items returns the root menu items
func (tm *TreeMenu) Items() []TreeMenuItem {
	return tm.items
}

// VisibleItems returns the currently visible items (at current drill level, filtered)
func (tm *TreeMenu) VisibleItems() []TreeMenuItem {
	return tm.visibleItems()
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

// ToggleBlink toggles the blink state for animated items
func (tm *TreeMenu) ToggleBlink() {
	tm.blinkState = !tm.blinkState
}

// SetRightSidePanel sets whether this is a right-side panel (applies width adjustment)
func (tm *TreeMenu) SetRightSidePanel(isRight bool) {
	tm.rightSidePanel = isRight
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

// SetRenameActive sets whether rename input is active
func (tm *TreeMenu) SetRenameActive(active bool) {
	tm.renameActive = active
	if active {
		// Start with current label
		if item := tm.SelectedItem(); item != nil {
			tm.renameText = item.Label
		}
	} else {
		tm.renameText = ""
	}
}

// IsRenameActive returns whether rename input is active
func (tm *TreeMenu) IsRenameActive() bool {
	return tm.renameActive
}

// RenameText returns the current rename text
func (tm *TreeMenu) RenameText() string {
	return tm.renameText
}

// SetRenameText sets the rename text
func (tm *TreeMenu) SetRenameText(text string) {
	tm.renameText = text
}

// AppendRenameText appends to rename text
func (tm *TreeMenu) AppendRenameText(s string) {
	tm.renameText += s
}

// BackspaceRenameText removes last character from rename text
func (tm *TreeMenu) BackspaceRenameText() {
	if len(tm.renameText) > 0 {
		tm.renameText = tm.renameText[:len(tm.renameText)-1]
	}
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

// SelectedItem returns the currently selected item, or nil if back item or none
func (tm *TreeMenu) SelectedItem() *TreeMenuItem {
	// If back item is selected, return nil
	if tm.hasBackItem() && tm.selectedIndex == 0 {
		return nil
	}

	items := tm.visibleItems()
	idx := tm.adjustedIndex()
	if idx < 0 || idx >= len(items) {
		return nil
	}
	return &items[idx]
}

// IsBackSelected returns true if the back item is currently selected
func (tm *TreeMenu) IsBackSelected() bool {
	return tm.hasBackItem() && tm.selectedIndex == 0
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

// MoveUp moves selection up, skipping disabled items
func (tm *TreeMenu) MoveUp() {
	if tm.selectedIndex > 0 {
		tm.selectedIndex--
		// Skip disabled items
		for tm.selectedIndex > 0 && tm.isIndexDisabled(tm.selectedIndex) {
			tm.selectedIndex--
		}
		// If landed on disabled, try to find next enabled going down
		if tm.isIndexDisabled(tm.selectedIndex) {
			tm.moveToNextEnabled(1)
		}
		// Adjust scroll to keep selection visible
		tm.ensureSelectionVisible()
	}
}

// MoveDown moves selection down, skipping disabled items
func (tm *TreeMenu) MoveDown() {
	total := tm.TotalVisibleCount()
	if tm.selectedIndex < total-1 {
		tm.selectedIndex++
		// Skip disabled items
		for tm.selectedIndex < total-1 && tm.isIndexDisabled(tm.selectedIndex) {
			tm.selectedIndex++
		}
		// If landed on disabled, try to find next enabled going up
		if tm.isIndexDisabled(tm.selectedIndex) {
			tm.moveToNextEnabled(-1)
		}
		// Adjust scroll to keep selection visible
		tm.ensureSelectionVisible()
	}
}

// PageUp moves selection up by one page
func (tm *TreeMenu) PageUp() {
	pageSize := tm.visibleRowCount()
	if pageSize <= 0 {
		pageSize = 10
	}
	tm.selectedIndex -= pageSize
	if tm.selectedIndex < 0 {
		tm.selectedIndex = 0
	}
	// Skip disabled items
	if tm.isIndexDisabled(tm.selectedIndex) {
		tm.moveToNextEnabled(1)
	}
	tm.ensureSelectionVisible()
}

// PageDown moves selection down by one page
func (tm *TreeMenu) PageDown() {
	total := tm.TotalVisibleCount()
	pageSize := tm.visibleRowCount()
	if pageSize <= 0 {
		pageSize = 10
	}
	tm.selectedIndex += pageSize
	if tm.selectedIndex >= total {
		tm.selectedIndex = total - 1
	}
	if tm.selectedIndex < 0 {
		tm.selectedIndex = 0
	}
	// Skip disabled items
	if tm.isIndexDisabled(tm.selectedIndex) {
		tm.moveToNextEnabled(-1)
	}
	tm.ensureSelectionVisible()
}

// ensureSelectionVisible adjusts scrollOffset to keep selectedIndex visible
func (tm *TreeMenu) ensureSelectionVisible() {
	visibleRows := tm.visibleRowCount()
	if visibleRows <= 0 {
		return
	}

	// Scroll up if selection is above visible area
	if tm.selectedIndex < tm.scrollOffset {
		tm.scrollOffset = tm.selectedIndex
	}

	// Scroll down if selection is below visible area
	if tm.selectedIndex >= tm.scrollOffset+visibleRows {
		tm.scrollOffset = tm.selectedIndex - visibleRows + 1
	}

	// Clamp scrollOffset
	if tm.scrollOffset < 0 {
		tm.scrollOffset = 0
	}
}

// visibleRowCount returns the number of item rows that can be displayed
func (tm *TreeMenu) visibleRowCount() int {
	if tm.height <= 0 {
		return 100 // No limit if height not set
	}
	// Account for: header (1) + separator (1) + search bar (2 if active) + border (2)
	overhead := 4
	if tm.searchActive || tm.searchQuery != "" {
		overhead += 2
	}
	rows := tm.height - overhead
	if rows < 1 {
		rows = 1
	}
	return rows
}

// isIndexDisabled checks if the item at the given index is disabled
func (tm *TreeMenu) isIndexDisabled(index int) bool {
	items := tm.visibleItems()
	// Account for parent back item
	offset := 0
	if tm.hasBackItem() {
		if index == 0 {
			return false // Back item is never disabled
		}
		offset = 1
	}
	actualIndex := index - offset
	if actualIndex < 0 || actualIndex >= len(items) {
		return false
	}
	return items[actualIndex].Disabled
}

// MoveAwayFromDisabled moves selection away from disabled items (call after disabling)
func (tm *TreeMenu) MoveAwayFromDisabled() {
	if tm.isIndexDisabled(tm.selectedIndex) {
		tm.moveToNextEnabled(1) // Try down first
		if tm.isIndexDisabled(tm.selectedIndex) {
			tm.moveToNextEnabled(-1) // Then try up
		}
	}
}

// moveToNextEnabled finds the next non-disabled item in the given direction
func (tm *TreeMenu) moveToNextEnabled(direction int) {
	total := tm.TotalVisibleCount()
	for i := 0; i < total; i++ {
		tm.selectedIndex += direction
		if tm.selectedIndex < 0 {
			tm.selectedIndex = 0
			return
		}
		if tm.selectedIndex >= total {
			tm.selectedIndex = total - 1
			return
		}
		if !tm.isIndexDisabled(tm.selectedIndex) {
			return
		}
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
	tm.scrollOffset = 0 // Reset scroll when drilling down
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
	tm.scrollOffset = 0 // Reset scroll when drilling up
	tm.searchQuery = "" // Clear search when drilling up
	return true
}

// Select activates the current item (drill-down if has children, otherwise return the item)
// Returns the selected leaf item, or nil if drilled down/up
func (tm *TreeMenu) Select() *TreeMenuItem {
	// Back item selected - drill up
	if tm.IsBackSelected() {
		tm.DrillUp()
		return nil
	}

	item := tm.SelectedItem()
	if item == nil {
		return nil
	}
	// Check if it has children - drill down
	if len(item.Children) > 0 {
		tm.DrillDown()
		return nil
	}
	// Check if it's a container (e.g., project) with no children - don't treat as leaf
	if dataStr, ok := item.Data.(string); ok && dataStr == "project" {
		// It's a project with no sessions - don't return as leaf
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
	const minWidth = 36
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

// hasBackItem returns true if there's a back item at index 0
func (tm *TreeMenu) hasBackItem() bool {
	return len(tm.drillDownPath) > 0
}

// adjustedIndex returns the real item index accounting for back item
func (tm *TreeMenu) adjustedIndex() int {
	if tm.hasBackItem() {
		return tm.selectedIndex - 1
	}
	return tm.selectedIndex
}

// TotalVisibleCount returns total items including back item
func (tm *TreeMenu) TotalVisibleCount() int {
	count := len(tm.visibleItems())
	if tm.hasBackItem() {
		count++
	}
	return count
}

// Render renders the tree menu
func (tm *TreeMenu) Render() string {
	width := tm.width
	if width == 0 {
		width = tm.CalcWidth()
	}

	// Adjust render width to prevent right border cutoff (only for right-side panels)
	renderWidth := width
	if tm.rightSidePanel {
		renderWidth = width - 5
		if renderWidth < 20 {
			renderWidth = 20
		}
	}

	items := tm.visibleItems()
	parent := tm.currentParent()
	innerWidth := renderWidth - 2  // Account for border (1 left + 1 right)
	contentWidth := innerWidth - 4 // Account for padding (2 left + 2 right)

	var lines []string

	// Header (title only, back is now a selectable item)
	headerStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Padding(0, 2).
		Width(innerWidth)
	lines = append(lines, headerStyle.Render(tm.title))

	// Separator
	lines = append(lines, lipgloss.NewStyle().
		Foreground(ColorBorder).
		Padding(0, 2).
		Width(innerWidth).
		Render(strings.Repeat("─", contentWidth)))

	// Search bar (if active or has query)
	if tm.searchActive || tm.searchQuery != "" {
		searchStyle := lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 2).Width(innerWidth)
		searchIcon := "🔍 "
		searchText := tm.searchQuery
		if tm.searchActive {
			searchText += "█"
		}
		if searchText == "" && !tm.searchActive {
			searchText = "type to search..."
		}
		searchLine := searchIcon + searchText
		lines = append(lines, searchStyle.Render(truncate(searchLine, contentWidth)))
		lines = append(lines, lipgloss.NewStyle().
			Foreground(ColorBorder).
			Padding(0, 2).
			Width(innerWidth).
			Render(strings.Repeat("─", contentWidth)))
	}

	// Back item (when drilled down)
	if parent != nil {
		isBackSelected := tm.selectedIndex == 0 && tm.focused
		cursor := "  "
		if isBackSelected {
			cursor = "▶ "
		}
		backLine := cursor + "← " + truncate(parent.Label, contentWidth-6)

		var backStyle lipgloss.Style
		if isBackSelected {
			backStyle = lipgloss.NewStyle().
				Bold(true).
				Background(ColorBgAlt).
				Foreground(ColorSecondary).
				Padding(0, 2).
				Width(innerWidth)
		} else {
			backStyle = lipgloss.NewStyle().
				Foreground(ColorSecondary).
				Padding(0, 2).
				Width(innerWidth)
		}
		lines = append(lines, backStyle.Render(backLine))
	}

	// Calculate visible range for scrolling
	totalItems := len(items)
	if parent != nil {
		totalItems++ // Account for back item
	}
	visibleRows := tm.visibleRowCount()

	// Ensure scrollOffset is valid
	if tm.scrollOffset < 0 {
		tm.scrollOffset = 0
	}
	maxScroll := totalItems - visibleRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	if tm.scrollOffset > maxScroll {
		tm.scrollOffset = maxScroll
	}

	// Show scroll-up indicator
	if tm.scrollOffset > 0 {
		scrollUpStyle := lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 2).Width(innerWidth).Align(lipgloss.Center)
		lines = append(lines, scrollUpStyle.Render("▲ more"))
	}

	// Items
	if len(items) == 0 && parent == nil {
		emptyStyle := lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 2).Width(innerWidth)
		if tm.searchQuery != "" {
			lines = append(lines, emptyStyle.Render("No matches"))
		} else {
			lines = append(lines, emptyStyle.Render("No items"))
		}
	} else {
		// Determine which items to render based on scroll
		startIdx := tm.scrollOffset
		endIdx := tm.scrollOffset + visibleRows
		if tm.scrollOffset > 0 {
			endIdx-- // Account for scroll-up indicator
		}
		if endIdx > totalItems {
			endIdx = totalItems
		}

		for displayIndex := startIdx; displayIndex < endIdx; displayIndex++ {
			// Handle back item at index 0
			if parent != nil && displayIndex == 0 {
				// Back item is handled above, skip if in scroll range
				continue
			}

			// Convert displayIndex to item index
			itemIndex := displayIndex
			if parent != nil {
				itemIndex = displayIndex - 1
			}
			if itemIndex < 0 || itemIndex >= len(items) {
				continue
			}

			item := items[itemIndex]
			isSelected := displayIndex == tm.selectedIndex
			hasChildren := len(item.Children) > 0

			// Determine background for selected state (used for nested styles)
			var selectedBg lipgloss.TerminalColor
			if isSelected && tm.focused {
				selectedBg = ColorBgAlt
			}

			// Helper to apply background to text when selected
			withBg := func(text string) string {
				if selectedBg != nil {
					return lipgloss.NewStyle().Background(selectedBg).Render(text)
				}
				return text
			}

			// Build the line
			icon := item.Icon
			iconPart := ""
			if icon != "" {
				// Handle blinking: hide icon when blink is true and blinkState is false
				if item.Blink && !tm.blinkState {
					iconPart = withBg("  ") // Space instead of icon
				} else if item.IconColor != nil {
					iconStyle := lipgloss.NewStyle().Foreground(item.IconColor)
					if selectedBg != nil {
						iconStyle = iconStyle.Background(selectedBg)
					}
					iconPart = iconStyle.Render(icon + " ")
				} else {
					iconPart = withBg(icon + " ")
				}
			}

			// Cursor/indicator: ▶ for selected/active, space otherwise
			indicator := withBg("  ")
			if isSelected && tm.focused {
				indicator = withBg("▶ ")
			} else if item.IsActive {
				indicator = withBg("▶ ")
			}

			// Label with optional count
			label := item.Label
			countSuffix := ""
			if item.Count > 0 || (hasChildren && len(item.Children) > 0) {
				count := item.Count
				if count == 0 {
					count = len(item.Children)
				}
				countStyle := lipgloss.NewStyle().Foreground(ColorMuted)
				if selectedBg != nil {
					countStyle = countStyle.Background(selectedBg)
				}
				countSuffix = countStyle.Render(" (" + itoa(count) + ")")
			}

			// Trailing icon (e.g., ⚡ for attached terminal)
			trailingIcon := ""
			if item.TrailingIcon != "" {
				trailingIcon = withBg(" " + item.TrailingIcon)
			}

			// Arrow for items with children
			arrow := ""
			if hasChildren {
				arrow = withBg(" →")
			}

			// Calculate available space for label
			// Use lipgloss.Width for accurate display width (handles emojis, unicode)
			prefixWidth := lipgloss.Width(indicator) + lipgloss.Width(iconPart)
			suffixWidth := lipgloss.Width(countSuffix) + lipgloss.Width(trailingIcon) + lipgloss.Width(arrow)
			availableForLabel := contentWidth - prefixWidth - suffixWidth
			if availableForLabel < 10 {
				availableForLabel = 10
			}

			// Handle shortcut labels ([X] syntax)
			// If color is supported: strip brackets and color the shortcut
			// If no color: keep brackets as-is
			displayLabel := label
			shortcutPos := -1
			if SupportsColoredShortcuts() {
				displayLabel, shortcutPos = StripShortcutBrackets(label)
			}

			// Show rename input if this item is being renamed
			if isSelected && tm.renameActive && !hasChildren {
				displayLabel = tm.renameText + "█"
				countSuffix = "" // Hide count when renaming
				trailingIcon = ""
				arrow = ""
				shortcutPos = -1 // No shortcut when renaming
			}

			// Truncate the label (no ANSI codes yet)
			displayLabel = truncate(displayLabel, availableForLabel)

			// Apply shortcut coloring after truncation (if color supported and shortcut visible)
			// Pass background color for selected items to preserve hover background
			// ApplyShortcutColorWithBg handles both cases: with shortcut and without
			displayLabel = ApplyShortcutColorWithBg(displayLabel, shortcutPos, selectedBg)

			line := indicator + iconPart + displayLabel + countSuffix + trailingIcon + arrow

			// Apply style
			var style lipgloss.Style
			if isSelected && tm.focused {
				style = lipgloss.NewStyle().
					Bold(true).
					Background(ColorBgAlt).
					Foreground(ColorText).
					Padding(0, 2).
					Width(innerWidth)
			} else if item.IsActive {
				// Active items: white text with bold (indicator ▶ shows active state)
				style = lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorText).
					Padding(0, 2).
					Width(innerWidth)
			} else {
				// Unfocused selected items use normal text color (only active is purple)
				style = lipgloss.NewStyle().
					Foreground(ColorText).
					Padding(0, 2).
					Width(innerWidth)
			}

			lines = append(lines, style.Render(line))
		}

		// Show scroll-down indicator
		if endIdx < totalItems {
			scrollDownStyle := lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 2).Width(innerWidth).Align(lipgloss.Center)
			lines = append(lines, scrollDownStyle.Render("▼ more"))
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
		Width(renderWidth).
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
