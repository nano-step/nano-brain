module Api
  module V1
    class PaymentsController < ApplicationController
      def index
        payments = Payment.all
        render json: payments
      end

      def billing
        result = PaymentProcessor.new.process(params[:amount])
        render json: result
      end

      def upcoming_month
        payments = Payment.where("due_date > ?", Date.today)
        render json: payments
      end
    end
  end
end
