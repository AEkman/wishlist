document.addEventListener("wishlist:ranking-pick", (event) => {
  const winner = event.target.closest(".pair-option");
  const pair = winner?.closest(".pair");
  if (!pair) return;
  const loser = Array.from(pair.querySelectorAll(".pair-option")).find((option) => option !== winner);
  const title = (option) => option.querySelector(".pick-title").textContent.trim();
  const { winnerBefore, winnerAfter, loserBefore, loserAfter } = event.detail;
  console.info(`[Ranking] Vald: ${title(winner)} ${winnerBefore} → ${winnerAfter}. Andra: ${title(loser)} ${loserBefore} → ${loserAfter}.`);
});
