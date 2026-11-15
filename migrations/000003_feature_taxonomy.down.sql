DELETE FROM anime_features WHERE feature_key IN (
'protagonist.overpowered','protagonist.underdog','protagonist.antihero','protagonist.genius','protagonist.hidden_identity','protagonist.morally_ambiguous',
'power.progression','power.leveling_system','power.skill_tree','power.magic','power.supernatural',
'combat.strategic','combat.escalating_conflict',
'narrative.mystery','narrative.plot_twists','narrative.conspiracy','narrative.psychological','narrative.political_intrigue',
'theme.survival','theme.revenge','theme.war',
'world.dark_fantasy','world.dystopian','world.medieval','world.post_apocalyptic',
'pacing.fast','pacing.slow_burn','pacing.episodic',
'tone.dark','tone.tragic','tone.hopeful','tone.comedic',
'stakes.high','content.graphic_violence','content.gore','content.fan_service',
'ending.shocking_revelation','ending.bittersweet','ending.open_ended','ending.conclusive'
);
