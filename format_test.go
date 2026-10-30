package main

import "testing"

func TestFormat(t *testing.T) {
	n := " "
	cases := []struct{ in, want string }{
		{"bonjour virgule monde point à la ligne ça va point d'interrogation",
			"Bonjour, monde.\nÇa va" + n + "?"},
		// whisper's own punctuation around commands is dropped
		{"Bonjour, virgule, Maître Dupont. Point. À la ligne.", "Bonjour, Maître Dupont."},
		{"voici deux points ouvrez les guillemets oui fermez les guillemets point",
			"Voici" + n + ": «" + n + "oui" + n + "»."},
		{"il dit point-virgule puis nouveau paragraphe suite", "Il dit" + n + "; puis\n\nSuite"},
		{"ouvrez la parenthèse voir ci-dessous fermez la parenthèse point", "(Voir ci-dessous)."},
		// abbreviations keep their period and do not end the sentence
		{"M. Dupont et Mme Martin point", "M. Dupont et Mme Martin."},
		{"article L 123-4 du code", "Article L." + n + "123-4 du code"},
		{"le contrat de l'article L. 123-4 point", "Le contrat de l'article L." + n + "123-4."},
		// numbers
		{"mille deux cents euros", "1" + n + "200" + n + "€"},
		{"le douze mars deux mille vingt-six", "Le 12 mars 2026"},
		{"le premier mars", "Le 1er mars"},
		{"quatre-vingt-dix-sept pour cent", "97" + n + "%"},
		{"deux parties et un contrat", "Deux parties et un contrat"},
		{"trois quatre", "Trois quatre"},
		{"vingt et un jours", "21 jours"},
		{"dix et Dupont", "10 et Dupont"},
		// whisper already turned "virgule" into a comma and wrote digits
		{"Bonjour, maître du Pont, à la ligne article L 123 42 1200 €.", "Bonjour, maître du Pont,\nArticle L." + n + "123 42 1" + n + "200" + n + "€."},
		{"Il paie 200 € maintenant", "Il paie 200" + n + "€ maintenant"},
		{"Dupont. Point. À la ligne. Suite", "Dupont.\nSuite"},
		{"Dupont virgule. Martin", "Dupont, Martin"},
		{"(musique) bonjour [silence]", "Bonjour"},
		{"", ""},
	}
	for _, c := range cases {
		if got := Format(c.in); got != c.want {
			t.Errorf("Format(%q)\n got  %q\n want %q", c.in, got, c.want)
		}
	}
}
