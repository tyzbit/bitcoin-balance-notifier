$(document).ready(function () {
  function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }
  function refreshAddresses() {
    // Populate addresses
    $.get("/balances", function (data) {
      options = "";
      if (data.addresses) {
        for (let i = 0; i < data.addresses.length; i++) {
          options =
            options +
            `<option value="${data.addresses[i].Address}">${data.addresses[i].Nickname}" (address)</option>`;
        }
      }
      if (data.pubkeys) {
        for (let i = 0; i < data.pubkeys.length; i++) {
          options =
            options +
            `<option value="${data.pubkeys[i].Pubkey}">${data.pubkeys[i].Nickname} (pubkey)</option>`;
        }
      }
      $("#addresses").html(options);
    });
  }

  function getAddressDetails() {
    value = $("#addresses :selected").val();
    $.post("/balance", JSON.stringify({ Identifier: value })).done(function (
      data
    ) {
      resp = data.reqInfo;
      address = "";
      if (resp.Address != null) {
        address = resp.Address;
      } else {
        address = resp.Pubkey;
      }
      entry = `<div class="address-entry">
        <b>Address: </b>${address}<br>
        <b>Balance: </b>${resp.BalanceSat} satoshis<br>
        <b>Previous Balance: </b>${resp.PreviousBalanceSat} satoshis<br>
        <b>Value: </b>${resp.BalanceCurrency} ${resp.Currency}<br>
        <b>Previous Value: </b>${resp.PreviousBalanceCurrency} ${resp.Currency}<br>
        <b>Transactions: </b>${resp.TXCount}<br>
        <button id="remove">Remove this address</button>
        <p id="delete-status"></p>
      </div>`;
      $("#address-info").html(entry);
    });
  }
  refreshAddresses();

  $("#addresses").click(function (e) {
    if (e.target.tagName == "SELECT") {
      $("#addresses").find(":selected").prop("selected", false);
      $("#address-info").html("");
      return;
    }
    getAddressDetails();
  });

  $("#add").click(function () {
    identifier = $("#identifier").val();
    nickname = $("#nickname").val();
    $.post(
      "/watch",
      JSON.stringify({ Identifier: identifier, Nickname: nickname })
    ).always(function (data) {
      message = "Success";
      if (data.responseJSON != null && data.responseJSON.errors) {
        message = data.responseJSON.errors;
      } else {
        refreshAddresses();
      }
      $("#add-status").html(message).css("opacity", "100%");
      $("#add-status").delay(2000).animate({ opacity: "40%" });
    });
  });

  $(document).on("click", "#remove", function () {
    identifier = $("#addresses :selected").val();
    $.ajax({
      type: "DELETE",
      url: "/identifier",
      data: JSON.stringify({ Identifier: identifier }),
    }).done(function (data) {
      var message;
      if (data) {
        message = "Success";
      } else {
        message = "Failure";
      }
      $("#delete-status").html(message);
      entry = `<div class="address-entry"><br></div>`;
      $("#address-info").html(entry);
      refreshAddresses();
    });
  });
});
