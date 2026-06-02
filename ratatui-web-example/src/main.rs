use std::{cell::RefCell, io, rc::Rc};

use ratzilla::ratatui::{
    layout::{Alignment, Constraint, Direction, Layout},
    style::{Color, Modifier, Style},
    text::Span,
    widgets::{Block, BorderType, Borders, Paragraph, Table, TableCell, Row as TableRow},
    Terminal,
};
use ratzilla::{event::KeyCode, DomBackend, WebRenderer};

#[derive(Default)]
struct AppState {
    selected_index: usize,
    messages: Vec<String>,
}

fn main() -> io::Result<()> {
    let state = Rc::new(RefCell::new(AppState::default()));
    let backend = DomBackend::new()?;
    let mut terminal = Terminal::new(backend)?;

    // Handle keyboard input
    terminal.on_key_event({
        let state_cloned = state.clone();
        move |key_event| {
            let mut st = state_cloned.borrow_mut();
            match key_event.code {
                KeyCode::Char('q') => {
                    std::process::exit(0);
                }
                KeyCode::Up | KeyCode::Char('k') => {
                    if !st.messages.is_empty() {
                        st.selected_index = st.selected_index.saturating_sub(1);
                    }
                }
                KeyCode::Down | KeyCode::Char('j') => {
                    if !st.messages.is_empty() {
                        st.selected_index = (st.selected_index + 1).min(st.messages.len().saturating_sub(1));
                    }
                }
                _ => {}
            }
        }
    });

    // Main render loop — runs every frame in the browser
    terminal.draw_web(move |f| {
        let st = state.borrow();

        // Split into header + main area
        let chunks = Layout::default()
            .direction(Direction::Vertical)
            .constraints([Constraint::Length(3), Constraint::Min(1)].as_ref())
            .split(f.area());

        // Header paragraph
        let header = Paragraph::new(Span::styled(
            "  Ratatui Web Example — Arrow keys to navigate, 'q' to quit  ",
            Style::default()
                .fg(Color::White)
                .add_modifier(Modifier::BOLD),
        ))
        .alignment(Alignment::Center)
        .block(
            Block::default()
                .borders(Borders::ALL)
                .border_type(BorderType::Rounded)
                .title(" Demo App ")
                .title_alignment(Alignment::Center)
                .style(Style::default().fg(Color::Cyan)),
        );

        f.render_widget(header, chunks[0]);

        // Message list (simulated — shows a few static messages)
        let msgs = if st.messages.is_empty() {
            vec![
                "Hello! This is a Ratatui web app.".to_string(),
                "Press ↑/↓ or k/j to navigate.".to_string(),
                "Press q to quit.".to_string(),
                "Built with Ratatui + Ratzilla + WebAssembly.".to_string(),
            ]
        } else {
            st.messages.clone()
        };

        let table_rows: Vec<TableRow> = msgs
            .iter()
            .enumerate()
            .map(|(i, msg)| {
                let style = if i == st.selected_index {
                    Style::default().fg(Color::Yellow).add_modifier(Modifier::BOLD)
                } else {
                    Style::default().fg(Color::Gray)
                };
                vec![TableCell::new(msg.as_str()).style(style)]
            })
            .collect();

        let table = Table::new(table_rows, [Constraint::Percentage(100)])
            .header(TableRow::from(vec![TableCell::new(" Messages ").style(
                Style::default().fg(Color::White).add_modifier(Modifier::BOLD),
            )])]
            .block(
                Block::default()
                    .borders(Borders::ALL)
                    .border_type(BorderType::Rounded)
                    .title(" Message List ")
                    .style(Style::default().fg(Color::Yellow)),
            );

        f.render_widget(table, chunks[1]);
    });

    Ok(())
}
